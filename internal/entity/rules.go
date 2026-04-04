package entity

// Cost is the resource requirement for training a unit or constructing a building.
type Cost struct {
	Food  int
	Gold  int
	Stone int
	Wood  int
}

// UnitCosts are the current training costs for each unit kind.
var UnitCosts = map[UnitKind]Cost{
	KindVillager:     {Food: 50},
	KindInfantry:     {Food: 60, Gold: 20},
	KindSpearman:     {Food: 50, Wood: 20},
	KindScoutCavalry: {Food: 70, Gold: 30},
	KindPaladin:      {Food: 90, Gold: 70},
	KindArcher:       {Food: 40, Wood: 30},
}

// BuildingCosts are the current construction costs for each buildable structure.
var BuildingCosts = map[BuildingKind]Cost{
	KindBarracks:     {Wood: 140},
	KindStable:       {Wood: 160, Stone: 40},
	KindArcheryRange: {Wood: 130, Gold: 20},
}

// ParseUnitKind converts an API string into a unit kind.
func ParseUnitKind(s string) (UnitKind, bool) {
	for kind, name := range unitKindNames {
		if name == s {
			return kind, true
		}
	}
	return 0, false
}

// ParseBuildingKind converts an API string into a building kind.
func ParseBuildingKind(s string) (BuildingKind, bool) {
	for kind, name := range buildingKindNames {
		if name == s {
			return kind, true
		}
	}
	return 0, false
}

// UnitProducer reports whether a building can train a given unit kind.
func UnitProducer(kind UnitKind) BuildingKind {
	switch kind {
	case KindVillager:
		return KindTownCenter
	case KindInfantry, KindSpearman:
		return KindBarracks
	case KindScoutCavalry, KindPaladin:
		return KindStable
	case KindArcher:
		return KindArcheryRange
	default:
		return KindTownCenter
	}
}

// AttackRange returns the unit's current attack range in hexes.
func AttackRange(kind UnitKind) int {
	if kind == KindArcher {
		return 2
	}
	return 1
}

// CounterBonus returns the additive damage bonus in the matchup.
func CounterBonus(attacker, defender UnitKind) int {
	switch {
	case attacker == KindSpearman &&
		(defender == KindScoutCavalry || defender == KindPaladin):
		return 8
	case attacker == KindArcher && defender == KindSpearman:
		return 4
	case attacker == KindScoutCavalry && defender == KindArcher:
		return 4
	default:
		return 0
	}
}
