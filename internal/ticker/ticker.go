package ticker

import (
	"log/slog"
	"time"

	"github.com/jason9075/agents_of_dynasties/internal/entity"
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

// step processes one game tick: drains the command queue, applies commands,
// then increments the world tick counter.
func (t *Ticker) step() {
	cmds := t.queue.Drain()

	tick := t.world.GetTick() + 1
	slog.Info("tick", "tick", tick, "commands", len(cmds))

	for _, cmd := range cmds {
		slog.Debug("command",
			"tick", tick,
			"team", cmd.Team,
			"unit_id", cmd.UnitID,
			"building_id", cmd.BuildingID,
			"kind", cmd.Kind,
		)
		t.applyCommand(cmd)
	}

	t.world.ProcessProduction()
	t.world.IncrementTick()
}

func (t *Ticker) applyCommand(cmd Command) {
	switch cmd.Kind {
	case CmdMoveFast, CmdMoveGuard:
		if cmd.TargetCoord == nil {
			return
		}
		u := t.world.GetUnit(cmd.UnitID)
		if u == nil {
			return
		}
		speed := u.Stats().SpeedFast
		if cmd.Kind == CmdMoveGuard {
			speed = u.Stats().SpeedGuard
		}
		t.world.MoveUnitToward(cmd.UnitID, *cmd.TargetCoord, speed)
	case CmdGather:
		t.world.GatherAtCurrentTile(cmd.UnitID)
	case CmdBuild:
		if cmd.TargetCoord == nil || cmd.BuildingKind == nil {
			return
		}
		kind, ok := entity.ParseBuildingKind(*cmd.BuildingKind)
		if !ok {
			return
		}
		t.world.BuildStructure(cmd.UnitID, kind, *cmd.TargetCoord)
	case CmdAttack:
		if cmd.TargetID == nil {
			return
		}
		t.world.AttackTarget(cmd.UnitID, *cmd.TargetID)
	case CmdProduce:
		if cmd.BuildingID == nil || cmd.UnitKind == nil {
			return
		}
		kind, ok := entity.ParseUnitKind(*cmd.UnitKind)
		if !ok {
			return
		}
		t.world.EnqueueProduction(*cmd.BuildingID, kind)
	}
}
