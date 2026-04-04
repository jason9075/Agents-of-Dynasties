package world

import (
	"testing"

	"github.com/jason9075/agents_of_dynasties/internal/entity"
	"github.com/jason9075/agents_of_dynasties/internal/hex"
	"github.com/jason9075/agents_of_dynasties/internal/terrain"
)

func TestGatherAtCurrentTile_VillagerAddsResources(t *testing.T) {
	w := NewWorld(42)
	villager := w.UnitsByTeam(entity.Team1)[0]
	start := w.GetResources(entity.Team1)

	w.WriteFunc(func() {
		villager.SetPosition(hex.Coord{Q: 3, R: 8})
		w.Tiles[hex.Coord{Q: 3, R: 8}] = terrain.Tile{Coord: hex.Coord{Q: 3, R: 8}, Terrain: terrain.GoldMine}
	})

	if !w.GatherAtCurrentTile(villager.ID()) {
		t.Fatalf("expected villager gather to succeed")
	}

	after := w.GetResources(entity.Team1)
	if after.Gold != start.Gold+20 {
		t.Fatalf("gold = %d, want %d", after.Gold, start.Gold+20)
	}
}

func TestBuildStructure_VillagerBuildsBarracks(t *testing.T) {
	w := NewWorld(42)
	villager := w.UnitsByTeam(entity.Team1)[0]
	target := hex.Coord{Q: 6, R: 5}
	start := w.GetResources(entity.Team1)

	w.WriteFunc(func() {
		villager.SetPosition(hex.Coord{Q: 5, R: 5})
		w.Tiles[target] = terrain.Tile{Coord: target, Terrain: terrain.Plain}
	})

	if !w.BuildStructure(villager.ID(), entity.KindBarracks, target) {
		t.Fatalf("expected villager build to succeed")
	}

	found := false
	for _, b := range w.BuildingsByTeam(entity.Team1) {
		if b.Kind() == entity.KindBarracks && b.Position() == target {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected barracks at %v", target)
	}

	after := w.GetResources(entity.Team1)
	if after.Wood != start.Wood-entity.BuildingCosts[entity.KindBarracks].Wood {
		t.Fatalf("wood = %d, want %d", after.Wood, start.Wood-entity.BuildingCosts[entity.KindBarracks].Wood)
	}
}

func TestAttackTarget_SpearmanCountersPaladin(t *testing.T) {
	w := NewWorld(42)
	attacker := w.SpawnUnit(entity.Team1, entity.KindSpearman, hex.Coord{Q: 8, R: 7})
	target := w.SpawnUnit(entity.Team2, entity.KindPaladin, hex.Coord{Q: 9, R: 7})

	if !w.AttackTarget(attacker.ID(), target.ID()) {
		t.Fatalf("expected attack to succeed")
	}

	got := w.GetUnit(target.ID()).HP()
	want := entity.DefaultStats[entity.KindPaladin].MaxHP - (entity.DefaultStats[entity.KindSpearman].Attack + 8 - entity.DefaultStats[entity.KindPaladin].Defense)
	if got != want {
		t.Fatalf("paladin hp = %d, want %d", got, want)
	}
}

func TestEnqueueProduction_AndProcessProduction(t *testing.T) {
	w := NewWorld(42)
	barracks := w.SpawnBuilding(entity.Team1, entity.KindBarracks, hex.Coord{Q: 8, R: 7})
	startUnits := len(w.UnitsByTeam(entity.Team1))
	start := w.GetResources(entity.Team1)

	if !w.EnqueueProduction(barracks.ID(), entity.KindInfantry) {
		t.Fatalf("expected enqueue production to succeed")
	}

	w.ProcessProduction()

	afterUnits := len(w.UnitsByTeam(entity.Team1))
	if afterUnits != startUnits+1 {
		t.Fatalf("units = %d, want %d", afterUnits, startUnits+1)
	}

	after := w.GetResources(entity.Team1)
	cost := entity.UnitCosts[entity.KindInfantry]
	if after.Food != start.Food-cost.Food || after.Gold != start.Gold-cost.Gold {
		t.Fatalf("resources after production = %+v, want food=%d gold=%d", after, start.Food-cost.Food, start.Gold-cost.Gold)
	}
}
