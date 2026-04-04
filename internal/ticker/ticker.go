package ticker

import (
	"log/slog"
	"sort"
	"time"

	"github.com/jason9075/agents_of_dynasties/internal/entity"
	"github.com/jason9075/agents_of_dynasties/internal/hex"
	"github.com/jason9075/agents_of_dynasties/internal/world"
)

const DefaultInterval = 10 * time.Second

// Ticker drives the game loop, processing commands every interval.
type Ticker struct {
	world    *world.World
	queue    *Queue
	interval time.Duration
	stop     chan struct{}
	done     chan struct{}
}

// New creates a Ticker. Call Start to begin the game loop.
func New(w *world.World, q *Queue, interval time.Duration) *Ticker {
	return &Ticker{
		world:    w,
		queue:    q,
		interval: interval,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start launches the game loop in a background goroutine.
func (t *Ticker) Start() {
	go t.loop()
}

// Step resolves exactly one game tick synchronously.
// Useful for tests and server-side sandbox simulations.
func (t *Ticker) Step() {
	t.step()
}

// Stop signals the game loop to exit and waits for it to finish.
func (t *Ticker) Stop() {
	close(t.stop)
	<-t.done
}

func (t *Ticker) loop() {
	defer close(t.done)
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			t.step()
		case <-t.stop:
			return
		}
	}
}

func (t *Ticker) step() {
	cmds := t.queue.Drain()

	tick := t.world.GetTick() + 1
	slog.Info("tick", "tick", tick, "commands", len(cmds))

	t.applySubmittedCommands(cmds, tick)
	t.resolveMovement()
	t.resolveGuardTransitions()
	t.resolveCombat()
	t.resolveEconomy()
	t.world.ProcessProduction()
	t.cleanupAttackStatuses()
	t.world.IncrementTick()
}

func (t *Ticker) applySubmittedCommands(cmds []Command, tick uint64) {
	for _, cmd := range cmds {
		slog.Debug("command",
			"tick", tick,
			"team", cmd.Team,
			"unit_id", cmd.UnitID,
			"building_id", cmd.BuildingID,
			"kind", cmd.Kind,
		)

		switch cmd.Kind {
		case CmdProduce:
			if cmd.BuildingID == nil || cmd.UnitKind == nil {
				continue
			}
			kind, ok := entity.ParseUnitKind(*cmd.UnitKind)
			if !ok {
				continue
			}
			t.world.EnqueueProduction(*cmd.BuildingID, kind)
		case CmdStop:
			if u := t.world.GetUnit(cmd.UnitID); u != nil {
				u.ClearStatus()
			}
		case CmdMoveFast:
			if u := t.world.GetUnit(cmd.UnitID); u != nil && cmd.TargetCoord != nil {
				u.SetMoveStatus(entity.StatusMovingFast, *cmd.TargetCoord)
			}
		case CmdMoveGuard:
			if u := t.world.GetUnit(cmd.UnitID); u != nil && cmd.TargetCoord != nil {
				u.SetMoveStatus(entity.StatusMovingGuard, *cmd.TargetCoord)
			}
		case CmdAttack:
			if u := t.world.GetUnit(cmd.UnitID); u != nil && cmd.TargetID != nil {
				u.SetAttackStatus(*cmd.TargetID)
			}
		case CmdGather:
			if u := t.world.GetUnit(cmd.UnitID); u != nil && cmd.TargetCoord != nil {
				u.SetGatherStatus(*cmd.TargetCoord)
			}
		case CmdBuild:
			if u := t.world.GetUnit(cmd.UnitID); u != nil && cmd.TargetCoord != nil && cmd.BuildingKind != nil {
				kind, ok := entity.ParseBuildingKind(*cmd.BuildingKind)
				if !ok {
					continue
				}
				u.SetBuildStatus(*cmd.TargetCoord, kind)
			}
		}
	}
}

func (t *Ticker) resolveMovement() {
	moveCmds := make(map[entity.EntityID]hex.Coord)
	remaining := make(map[entity.EntityID]int)
	maxSteps := 0

	for _, u := range t.allUnits() {
		target, speed, phase, ok := t.movementDirective(u)
		if !ok || speed <= 0 {
			continue
		}
		u.SetStatusPhase(phase)
		moveCmds[u.ID()] = target
		remaining[u.ID()] = speed
		if speed > maxSteps {
			maxSteps = speed
		}
	}

	stopped := make(map[entity.EntityID]bool)
	for step := 0; step < maxSteps; step++ {
		proposals := make(map[hex.Coord][]entity.EntityID)
		destByUnit := make(map[entity.EntityID]hex.Coord)

		for unitID, target := range moveCmds {
			if stopped[unitID] || remaining[unitID] <= 0 {
				continue
			}
			next, ok := t.world.PreviewMoveStep(unitID, target)
			if !ok {
				stopped[unitID] = true
				continue
			}
			proposals[next] = append(proposals[next], unitID)
			destByUnit[unitID] = next
		}

		accepted := make(map[entity.EntityID]hex.Coord)
		for dest, unitIDs := range proposals {
			if len(unitIDs) != 1 {
				for _, unitID := range unitIDs {
					stopped[unitID] = true
				}
				continue
			}
			accepted[unitIDs[0]] = dest
		}

		if len(accepted) == 0 {
			break
		}

		t.world.ApplyUnitMoves(accepted)
		for unitID := range accepted {
			remaining[unitID]--
			if remaining[unitID] <= 0 || destByUnit[unitID] == moveCmds[unitID] {
				stopped[unitID] = true
			}
		}
	}
}

func (t *Ticker) resolveGuardTransitions() {
	for _, u := range t.allUnits() {
		if u.Status() != entity.StatusMovingGuard {
			continue
		}
		if targetID, ok := t.world.FindAutoAttackTarget(u.ID()); ok {
			u.SetAttackStatus(targetID)
			u.SetStatusPhase(entity.PhaseAttacking)
		}
	}
}

func (t *Ticker) resolveCombat() {
	damage := make(map[entity.EntityID]int)
	for _, u := range t.allUnits() {
		if u.Status() != entity.StatusAttacking {
			continue
		}
		targetID, ok := u.StatusTargetID()
		if !ok {
			u.ClearStatus()
			continue
		}
		if amount, ok := t.world.PreviewAttackDamage(u.ID(), targetID); ok {
			u.SetStatusPhase(entity.PhaseAttacking)
			damage[targetID] += amount
			continue
		}
		if t.targetExists(targetID) {
			u.SetStatusPhase(entity.PhaseClosingToAttack)
			continue
		}
		u.ClearStatus()
	}

	if len(damage) > 0 {
		t.world.ApplyDamage(damage)
	}
}

func (t *Ticker) resolveEconomy() {
	for _, u := range t.allUnits() {
		switch u.Status() {
		case entity.StatusGathering:
			t.resolveGatherStatus(u)
		case entity.StatusBuilding:
			t.resolveBuildStatus(u)
		case entity.StatusMovingFast, entity.StatusMovingGuard:
			if target, ok := u.StatusTargetCoord(); ok && u.Position() == target {
				u.ClearStatus()
			}
		}
	}
}

func (t *Ticker) resolveGatherStatus(u *entity.Unit) {
	target, ok := u.StatusTargetCoord()
	if !ok {
		u.ClearStatus()
		return
	}

	if u.CarryAmount() > 0 {
		if t.world.CanDepositCarry(u.ID()) {
			if t.world.GatherAtCurrentTile(u.ID()) {
				if !t.world.IsGatherableResource(target) {
					u.ClearStatus()
					return
				}
				u.SetStatusPhase(entity.PhaseMovingToResource)
			}
			return
		}
		if _, ok := t.world.FindNearestFriendlyTownCenter(u.Team(), u.Position()); ok {
			u.SetStatusPhase(entity.PhaseReturning)
			return
		}
		u.ClearStatus()
		return
	}

	if !t.world.IsGatherableResource(target) {
		u.ClearStatus()
		return
	}
	if u.Position() != target {
		u.SetStatusPhase(entity.PhaseMovingToResource)
		return
	}
	if t.world.GatherAtCurrentTile(u.ID()) {
		if u.CarryAmount() > 0 {
			u.SetStatusPhase(entity.PhaseReturning)
			return
		}
	}
	if !t.world.IsGatherableResource(target) {
		u.ClearStatus()
	}
}

func (t *Ticker) resolveBuildStatus(u *entity.Unit) {
	target, ok := u.StatusTargetCoord()
	if !ok {
		u.ClearStatus()
		return
	}
	kind, ok := u.StatusBuildingKind()
	if !ok {
		u.ClearStatus()
		return
	}

	building := t.world.BuildingAt(target)
	if building != nil && building.Team() == u.Team() && building.Kind() == kind && building.IsComplete() {
		u.ClearStatus()
		return
	}
	if hex.Distance(u.Position(), target) > 1 {
		u.SetStatusPhase(entity.PhaseMovingToBuild)
		return
	}

	switch t.world.WorkOnBuild(u.ID(), kind, target) {
	case world.BuildActionWorking:
		u.SetStatusPhase(entity.PhaseConstructing)
	case world.BuildActionComplete:
		u.ClearStatus()
	case world.BuildActionBlocked:
		u.SetStatusPhase(entity.PhaseMovingToBuild)
	default:
		u.ClearStatus()
	}
}

func (t *Ticker) cleanupAttackStatuses() {
	for _, u := range t.allUnits() {
		if u.Status() != entity.StatusAttacking {
			continue
		}
		targetID, ok := u.StatusTargetID()
		if !ok || !t.targetExists(targetID) {
			u.ClearStatus()
		}
	}
}

func (t *Ticker) movementDirective(u *entity.Unit) (hex.Coord, int, entity.UnitStatusPhase, bool) {
	switch u.Status() {
	case entity.StatusMovingFast:
		target, ok := u.StatusTargetCoord()
		if !ok || u.Position() == target {
			return hex.Coord{}, 0, entity.PhaseNone, false
		}
		return target, u.Stats().SpeedFast, entity.PhaseMovingToTarget, true
	case entity.StatusMovingGuard:
		if _, ok := t.world.FindAutoAttackTarget(u.ID()); ok {
			return hex.Coord{}, 0, entity.PhaseAttacking, false
		}
		target, ok := u.StatusTargetCoord()
		if !ok || u.Position() == target {
			return hex.Coord{}, 0, entity.PhaseNone, false
		}
		return target, u.Stats().SpeedGuard, entity.PhaseMovingToTarget, true
	case entity.StatusAttacking:
		targetID, ok := u.StatusTargetID()
		if !ok {
			return hex.Coord{}, 0, entity.PhaseNone, false
		}
		if _, ok := t.world.PreviewAttackDamage(u.ID(), targetID); ok {
			return hex.Coord{}, 0, entity.PhaseAttacking, false
		}
		targetPos, ok := t.targetPosition(targetID)
		if !ok {
			return hex.Coord{}, 0, entity.PhaseNone, false
		}
		return targetPos, u.Stats().SpeedGuard, entity.PhaseClosingToAttack, true
	case entity.StatusGathering:
		target, ok := u.StatusTargetCoord()
		if !ok {
			return hex.Coord{}, 0, entity.PhaseNone, false
		}
		if u.CarryAmount() > 0 {
			tc, ok := t.world.FindNearestFriendlyTownCenter(u.Team(), u.Position())
			if !ok {
				return hex.Coord{}, 0, entity.PhaseNone, false
			}
			if t.world.CanDepositCarry(u.ID()) {
				return hex.Coord{}, 0, entity.PhaseDepositing, false
			}
			return tc, u.Stats().SpeedFast, entity.PhaseReturning, true
		}
		if !t.world.IsGatherableResource(target) || u.Position() == target {
			return hex.Coord{}, 0, entity.PhaseGathering, false
		}
		return target, u.Stats().SpeedFast, entity.PhaseMovingToResource, true
	case entity.StatusBuilding:
		target, ok := u.StatusTargetCoord()
		if !ok {
			return hex.Coord{}, 0, entity.PhaseNone, false
		}
		if hex.Distance(u.Position(), target) <= 1 {
			return hex.Coord{}, 0, entity.PhaseConstructing, false
		}
		moveTarget, ok := t.buildApproachTarget(u, target)
		if !ok {
			return hex.Coord{}, 0, entity.PhaseMovingToBuild, false
		}
		return moveTarget, u.Stats().SpeedFast, entity.PhaseMovingToBuild, true
	default:
		return hex.Coord{}, 0, entity.PhaseNone, false
	}
}

func (t *Ticker) buildApproachTarget(u *entity.Unit, target hex.Coord) (hex.Coord, bool) {
	bestDist := -1
	best := hex.Coord{}
	for _, candidate := range hex.Ring(target, 1) {
		if !hex.InBounds(candidate) {
			continue
		}
		if candidate == u.Position() {
			return candidate, true
		}
		tile, ok := t.world.Tile(candidate)
		if !ok || !entity.UnitCanEnterTerrain(u.Kind(), tile.Terrain) || !t.world.CanOccupy(candidate) {
			continue
		}
		dist := hex.Distance(u.Position(), candidate)
		if bestDist == -1 || dist < bestDist {
			best = candidate
			bestDist = dist
		}
	}
	return best, bestDist != -1
}

func (t *Ticker) targetPosition(id entity.EntityID) (hex.Coord, bool) {
	if u := t.world.GetUnit(id); u != nil && u.IsAlive() {
		return u.Position(), true
	}
	if b := t.world.GetBuilding(id); b != nil && b.IsAlive() {
		return b.Position(), true
	}
	return hex.Coord{}, false
}

func (t *Ticker) targetExists(id entity.EntityID) bool {
	_, ok := t.targetPosition(id)
	return ok
}

func (t *Ticker) allUnits() []*entity.Unit {
	units := append(t.world.UnitsByTeam(entity.Team1), t.world.UnitsByTeam(entity.Team2)...)
	sort.Slice(units, func(i, j int) bool { return units[i].ID() < units[j].ID() })
	return units
}
