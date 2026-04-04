package ticker

import (
	"testing"
	"time"

	"github.com/jason9075/agents_of_dynasties/internal/entity"
	"github.com/jason9075/agents_of_dynasties/internal/hex"
	"github.com/jason9075/agents_of_dynasties/internal/terrain"
	"github.com/jason9075/agents_of_dynasties/internal/world"
)

func TestStep_AppliesGatherBuildAndProduce(t *testing.T) {
	w := world.NewWorld(42)
	q := NewQueue()
	tk := New(w, q, time.Second)

	villagers := w.UnitsByTeam(entity.Team1)
	builder := villagers[0]
	gatherer := villagers[1]
	buildPos := hex.Coord{Q: 6, R: 5}
	resourcePos := hex.Coord{Q: 5, R: 5}
	startRes := w.GetResources(entity.Team1)

	w.WriteFunc(func() {
		builder.SetPosition(resourcePos)
		gatherer.SetPosition(resourcePos)
		w.Tiles[resourcePos] = terrain.Tile{Coord: resourcePos, Terrain: terrain.Orchard}
		w.Tiles[buildPos] = terrain.Tile{Coord: buildPos, Terrain: terrain.Plain}
	})

	q.Submit(Command{
		Team:   entity.Team1,
		UnitID: gatherer.ID(),
		Kind:   CmdGather,
	})
	tk.step()

	afterGather := w.GetResources(entity.Team1)
	if afterGather.Food != startRes.Food+25 {
		t.Fatalf("expected gather to add food, got %+v from %+v", afterGather, startRes)
	}

	buildingKind := "barracks"
	q.Submit(Command{
		Team:         entity.Team1,
		UnitID:       builder.ID(),
		Kind:         CmdBuild,
		TargetCoord:  &buildPos,
		BuildingKind: &buildingKind,
	})
	tk.step()

	var barracksID entity.EntityID
	for _, b := range w.BuildingsByTeam(entity.Team1) {
		if b.Kind() == entity.KindBarracks && b.Position() == buildPos {
			barracksID = b.ID()
		}
	}
	if barracksID == 0 {
		t.Fatalf("expected barracks to be built")
	}

	unitKind := "infantry"
	q.Submit(Command{
		Team:       entity.Team1,
		BuildingID: &barracksID,
		Kind:       CmdProduce,
		UnitKind:   &unitKind,
	})

	before := len(w.UnitsByTeam(entity.Team1))
	tk.step()
	after := len(w.UnitsByTeam(entity.Team1))

	if after != before+1 {
		t.Fatalf("expected produced infantry, units before=%d after=%d", before, after)
	}
}
