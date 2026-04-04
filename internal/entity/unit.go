package entity

import (
	"github.com/jason9075/agents_of_dynasties/internal/hex"
	"github.com/jason9075/agents_of_dynasties/internal/terrain"
)

// UnitKind identifies the type of a unit.
type UnitKind uint8

const (
	KindVillager     UnitKind = iota
	KindInfantry              // produced by Barracks
	KindSpearman              // produced by Barracks
	KindScoutCavalry          // produced by Stable
	KindPaladin               // produced by Stable
	KindArcher                // produced by Archery Range
)

func (k UnitKind) String() string {
	if spec, ok := UnitSpecs[k]; ok {
		return spec.Name
	}
	return "unknown_unit"
}

type UnitStatus string

const (
	StatusIdle        UnitStatus = "IDLE"
	StatusMovingFast  UnitStatus = "MOVING_FAST"
	StatusMovingGuard UnitStatus = "MOVING_GUARD"
	StatusAttacking   UnitStatus = "ATTACKING"
	StatusGathering   UnitStatus = "GATHERING"
	StatusBuilding    UnitStatus = "BUILDING"
)

type UnitStatusPhase string

const (
	PhaseNone             UnitStatusPhase = ""
	PhaseMovingToTarget   UnitStatusPhase = "MOVING_TO_TARGET"
	PhaseMovingToResource UnitStatusPhase = "MOVING_TO_RESOURCE"
	PhaseGathering        UnitStatusPhase = "GATHERING"
	PhaseReturning        UnitStatusPhase = "RETURNING"
	PhaseDepositing       UnitStatusPhase = "DEPOSITING"
	PhaseMovingToBuild    UnitStatusPhase = "MOVING_TO_BUILD"
	PhaseConstructing     UnitStatusPhase = "CONSTRUCTING"
	PhaseClosingToAttack  UnitStatusPhase = "CLOSING_TO_ATTACK"
	PhaseAttacking        UnitStatusPhase = "ATTACKING"
)

// Unit represents a mobile entity on the map.
type Unit struct {
	id                 EntityID
	team               Team
	kind               UnitKind
	pos                hex.Coord
	hp                 int
	carryType          terrain.ResourceType
	carryAmount        int
	status             UnitStatus
	statusPhase        UnitStatusPhase
	statusTargetCoord  *hex.Coord
	statusTargetID     *EntityID
	statusBuildingKind *BuildingKind
}

func NewUnit(id EntityID, team Team, kind UnitKind, pos hex.Coord) *Unit {
	stats := UnitSpecs[kind].Stats
	return &Unit{
		id:     id,
		team:   team,
		kind:   kind,
		pos:    pos,
		hp:     stats.MaxHP,
		status: StatusIdle,
	}
}

func (u *Unit) ID() EntityID                    { return u.id }
func (u *Unit) Team() Team                      { return u.team }
func (u *Unit) Position() hex.Coord             { return u.pos }
func (u *Unit) IsAlive() bool                   { return u.hp > 0 }
func (u *Unit) Kind() UnitKind                  { return u.kind }
func (u *Unit) HP() int                         { return u.hp }
func (u *Unit) MaxHP() int                      { return UnitSpecs[u.kind].Stats.MaxHP }
func (u *Unit) CarryType() terrain.ResourceType { return u.carryType }
func (u *Unit) CarryAmount() int                { return u.carryAmount }
func (u *Unit) Status() UnitStatus              { return u.status }
func (u *Unit) StatusPhase() UnitStatusPhase    { return u.statusPhase }
func (u *Unit) StatusTargetCoord() (hex.Coord, bool) {
	if u.statusTargetCoord == nil {
		return hex.Coord{}, false
	}
	return *u.statusTargetCoord, true
}
func (u *Unit) StatusTargetID() (EntityID, bool) {
	if u.statusTargetID == nil {
		return 0, false
	}
	return *u.statusTargetID, true
}
func (u *Unit) StatusBuildingKind() (BuildingKind, bool) {
	if u.statusBuildingKind == nil {
		return 0, false
	}
	return *u.statusBuildingKind, true
}
func (u *Unit) AttackTargetID() (EntityID, bool) {
	if u.status != StatusAttacking || u.statusTargetID == nil {
		return 0, false
	}
	return *u.statusTargetID, true
}
func (u *Unit) SetPosition(c hex.Coord) { u.pos = c }
func (u *Unit) SetHP(hp int)            { u.hp = hp }
func (u *Unit) SetCarry(rt terrain.ResourceType, amount int) {
	u.carryType = rt
	u.carryAmount = amount
}
func (u *Unit) ClearCarry() { u.carryType, u.carryAmount = terrain.ResourceNone, 0 }
func (u *Unit) SetMoveStatus(status UnitStatus, target hex.Coord) {
	targetCoord := target
	u.status = status
	u.statusTargetCoord = &targetCoord
	u.statusTargetID = nil
	u.statusBuildingKind = nil
	u.statusPhase = PhaseMovingToTarget
}
func (u *Unit) SetAttackStatus(id EntityID) {
	targetID := id
	u.status = StatusAttacking
	u.statusTargetID = &targetID
	u.statusTargetCoord = nil
	u.statusBuildingKind = nil
	u.statusPhase = PhaseClosingToAttack
}
func (u *Unit) SetGatherStatus(target hex.Coord) {
	targetCoord := target
	u.status = StatusGathering
	u.statusTargetCoord = &targetCoord
	u.statusTargetID = nil
	u.statusBuildingKind = nil
	u.statusPhase = PhaseMovingToResource
}
func (u *Unit) SetBuildStatus(target hex.Coord, kind BuildingKind) {
	targetCoord := target
	buildingKind := kind
	u.status = StatusBuilding
	u.statusTargetCoord = &targetCoord
	u.statusTargetID = nil
	u.statusBuildingKind = &buildingKind
	u.statusPhase = PhaseMovingToBuild
}
func (u *Unit) SetStatusPhase(phase UnitStatusPhase) { u.statusPhase = phase }
func (u *Unit) ClearStatus() {
	u.status = StatusIdle
	u.statusPhase = PhaseNone
	u.statusTargetCoord = nil
	u.statusTargetID = nil
	u.statusBuildingKind = nil
}
func (u *Unit) Stats() UnitStats { return UnitSpecs[u.kind].Stats }
