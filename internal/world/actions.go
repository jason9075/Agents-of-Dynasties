package world

import (
	"github.com/jason9075/agents_of_dynasties/internal/entity"
	"github.com/jason9075/agents_of_dynasties/internal/hex"
	"github.com/jason9075/agents_of_dynasties/internal/terrain"
)

// MoveUnitToward moves the unit toward target up to speed steps using greedy hex distance.
func (w *World) MoveUnitToward(unitID entity.EntityID, target hex.Coord, speed int) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	u := w.Units[unitID]
	if u == nil || !u.IsAlive() || speed <= 0 {
		return false
	}

	moved := false
	for step := 0; step < speed; step++ {
		cur := u.Position()
		if cur == target {
			break
		}

		best := cur
		bestDist := hex.Distance(cur, target)
		for _, candidate := range cur.Neighbors() {
			if !hex.InBounds(candidate) || !w.canOccupyLocked(candidate, unitID, 0) {
				continue
			}
			if dist := hex.Distance(candidate, target); dist < bestDist {
				best = candidate
				bestDist = dist
			}
		}

		if best == cur {
			break
		}
		u.SetPosition(best)
		moved = true
	}

	return moved
}

// GatherAtCurrentTile lets a villager harvest the resource on its current tile.
func (w *World) GatherAtCurrentTile(unitID entity.EntityID) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	u := w.Units[unitID]
	if u == nil || !u.IsAlive() || u.Kind() != entity.KindVillager {
		return false
	}

	tile, ok := w.Tiles[u.Position()]
	if !ok {
		return false
	}

	switch tile.Terrain.ResourceYield() {
	case terrain.ResourceFood:
		res := w.TeamRes[u.Team()]
		res.Food += 25
		w.TeamRes[u.Team()] = res
		return true
	case terrain.ResourceGold:
		res := w.TeamRes[u.Team()]
		res.Gold += 20
		w.TeamRes[u.Team()] = res
		return true
	case terrain.ResourceStone:
		res := w.TeamRes[u.Team()]
		res.Stone += 20
		w.TeamRes[u.Team()] = res
		return true
	case terrain.ResourceWood:
		res := w.TeamRes[u.Team()]
		res.Wood += 20
		w.TeamRes[u.Team()] = res
		return true
	default:
		return false
	}
}

// BuildStructure creates a structure at target if the villager is allowed and the team can pay the cost.
func (w *World) BuildStructure(builderID entity.EntityID, kind entity.BuildingKind, target hex.Coord) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	u := w.Units[builderID]
	if u == nil || !u.IsAlive() || u.Kind() != entity.KindVillager {
		return false
	}
	if kind == entity.KindTownCenter {
		return false
	}
	if !hex.InBounds(target) || hex.Distance(u.Position(), target) > 1 {
		return false
	}
	tile, ok := w.Tiles[target]
	if !ok || tile.Terrain != terrain.Plain {
		return false
	}
	if !w.canOccupyLocked(target, 0, 0) {
		return false
	}

	cost, ok := entity.BuildingCosts[kind]
	if !ok || !w.canAffordLocked(u.Team(), cost) {
		return false
	}
	w.payLocked(u.Team(), cost)

	b := entity.NewBuilding(w.nextID(), u.Team(), kind, target)
	w.Buildings[b.ID()] = b
	return true
}

// AttackTarget applies one attack from attacker to a target entity if in range.
func (w *World) AttackTarget(attackerID, targetID entity.EntityID) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	attacker := w.Units[attackerID]
	if attacker == nil || !attacker.IsAlive() {
		return false
	}

	targetUnit := w.Units[targetID]
	if targetUnit != nil && targetUnit.IsAlive() {
		if targetUnit.Team() == attacker.Team() {
			return false
		}
		if hex.Distance(attacker.Position(), targetUnit.Position()) > entity.AttackRange(attacker.Kind()) {
			return false
		}

		damage := attacker.Stats().Attack + entity.CounterBonus(attacker.Kind(), targetUnit.Kind()) - targetUnit.Stats().Defense
		if damage < 1 {
			damage = 1
		}
		targetUnit.SetHP(targetUnit.HP() - damage)
		if !targetUnit.IsAlive() {
			delete(w.Units, targetID)
		}
		return true
	}

	targetBuilding := w.Buildings[targetID]
	if targetBuilding == nil || !targetBuilding.IsAlive() || targetBuilding.Team() == attacker.Team() {
		return false
	}
	if hex.Distance(attacker.Position(), targetBuilding.Position()) > entity.AttackRange(attacker.Kind()) {
		return false
	}

	damage := attacker.Stats().Attack
	if damage < 1 {
		damage = 1
	}
	targetBuilding.SetHP(targetBuilding.HP() - damage)
	if !targetBuilding.IsAlive() {
		delete(w.Buildings, targetID)
	}
	return true
}

// EnqueueProduction adds a unit to a building queue if the team can pay and the producer matches.
func (w *World) EnqueueProduction(buildingID entity.EntityID, kind entity.UnitKind) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	b := w.Buildings[buildingID]
	if b == nil || !b.IsAlive() {
		return false
	}
	if entity.UnitProducer(kind) != b.Kind() {
		return false
	}
	cost, ok := entity.UnitCosts[kind]
	if !ok || !w.canAffordLocked(b.Team(), cost) {
		return false
	}

	w.payLocked(b.Team(), cost)
	b.Enqueue(kind)
	return true
}

// ProcessProduction spawns at most one queued unit per building each tick.
func (w *World) ProcessProduction() {
	w.mu.Lock()
	defer w.mu.Unlock()

	occupied := occupiedCoords(w)
	for _, b := range w.Buildings {
		if !b.IsAlive() || b.QueueLen() == 0 {
			continue
		}
		spawn, ok := findFirstOpenSpawnCoord(w, b.Position(), occupied)
		if !ok {
			continue
		}
		kind, ok := b.DequeueNext()
		if !ok {
			continue
		}
		u := entity.NewUnit(w.nextID(), b.Team(), kind, spawn)
		w.Units[u.ID()] = u
		occupied[spawn] = true
	}
}

func (w *World) canAffordLocked(team entity.Team, cost entity.Cost) bool {
	res := w.TeamRes[team]
	return res.Food >= cost.Food &&
		res.Gold >= cost.Gold &&
		res.Stone >= cost.Stone &&
		res.Wood >= cost.Wood
}

// CanAfford reports whether the team can pay the given cost.
func (w *World) CanAfford(team entity.Team, cost entity.Cost) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.canAffordLocked(team, cost)
}

func (w *World) payLocked(team entity.Team, cost entity.Cost) {
	res := w.TeamRes[team]
	res.Food -= cost.Food
	res.Gold -= cost.Gold
	res.Stone -= cost.Stone
	res.Wood -= cost.Wood
	w.TeamRes[team] = res
}

func (w *World) canOccupyLocked(c hex.Coord, ignoreUnitID, ignoreBuildingID entity.EntityID) bool {
	tile, ok := w.Tiles[c]
	if !ok || !tile.Terrain.Passable() {
		return false
	}
	for id, u := range w.Units {
		if id != ignoreUnitID && u.IsAlive() && u.Position() == c {
			return false
		}
	}
	for id, b := range w.Buildings {
		if id != ignoreBuildingID && b.IsAlive() && b.Position() == c {
			return false
		}
	}
	return true
}

// CanOccupy reports whether a coordinate is currently passable and unoccupied.
func (w *World) CanOccupy(c hex.Coord) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.canOccupyLocked(c, 0, 0)
}

func findFirstOpenSpawnCoord(w *World, origin hex.Coord, occupied map[hex.Coord]bool) (hex.Coord, bool) {
	for radius := 1; radius <= 3; radius++ {
		for _, c := range hex.Ring(origin, radius) {
			if !hex.InBounds(c) || occupied[c] {
				continue
			}
			tile, ok := w.Tiles[c]
			if !ok || !tile.Terrain.Passable() {
				continue
			}
			return c, true
		}
	}
	return hex.Coord{}, false
}
