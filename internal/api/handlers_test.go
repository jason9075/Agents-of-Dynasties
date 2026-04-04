package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jason9075/agents_of_dynasties/internal/entity"
	"github.com/jason9075/agents_of_dynasties/internal/hex"
	"github.com/jason9075/agents_of_dynasties/internal/terrain"
	"github.com/jason9075/agents_of_dynasties/internal/ticker"
	"github.com/jason9075/agents_of_dynasties/internal/world"
)

func TestCommandHandler_InvalidGatherReturnsCodeAndReason(t *testing.T) {
	w := world.NewWorld(42)
	q := ticker.NewQueue()
	h := &commandHandler{w: w, q: q}

	infantry := w.SpawnUnit(entity.Team1, entity.KindInfantry, hex.Coord{Q: 8, R: 7})
	body := map[string]any{
		"unit_id": infantry.ID(),
		"kind":    "GATHER",
	}

	rec := doCommandRequest(t, h, body, "1")
	assertErrorResponse(t, rec, http.StatusBadRequest, "unit_cannot_gather")
}

func TestCommandHandler_MoveOutOfBoundsReturnsCodeAndReason(t *testing.T) {
	w := world.NewWorld(42)
	q := ticker.NewQueue()
	h := &commandHandler{w: w, q: q}

	villager := w.UnitsByTeam(entity.Team1)[0]
	body := map[string]any{
		"unit_id": villager.ID(),
		"kind":    "MOVE_FAST",
		"target_coord": map[string]any{
			"q": 99,
			"r": 99,
		},
	}

	rec := doCommandRequest(t, h, body, "1")
	assertErrorResponse(t, rec, http.StatusBadRequest, "target_out_of_bounds")
}

func TestCommandHandler_AttackOutOfRangeReturnsCodeAndReason(t *testing.T) {
	w := world.NewWorld(42)
	q := ticker.NewQueue()
	h := &commandHandler{w: w, q: q}

	attacker := w.SpawnUnit(entity.Team1, entity.KindInfantry, hex.Coord{Q: 2, R: 2})
	target := w.SpawnUnit(entity.Team2, entity.KindArcher, hex.Coord{Q: 10, R: 10})
	body := map[string]any{
		"unit_id":   attacker.ID(),
		"kind":      "ATTACK",
		"target_id": target.ID(),
	}

	rec := doCommandRequest(t, h, body, "1")
	assertErrorResponse(t, rec, http.StatusBadRequest, "target_out_of_range")
}

func TestCommandHandler_InvalidProducerReturnsCodeAndReason(t *testing.T) {
	w := world.NewWorld(42)
	q := ticker.NewQueue()
	h := &commandHandler{w: w, q: q}

	var townCenterID entity.EntityID
	for _, b := range w.BuildingsByTeam(entity.Team1) {
		if b.Kind() == entity.KindTownCenter {
			townCenterID = b.ID()
		}
	}

	body := map[string]any{
		"building_id": townCenterID,
		"kind":        "PRODUCE",
		"unit_kind":   "infantry",
	}

	rec := doCommandRequest(t, h, body, "1")
	assertErrorResponse(t, rec, http.StatusBadRequest, "invalid_producer")
}

func TestCommandHandler_AccountsForPendingResourceReservations(t *testing.T) {
	w := world.NewWorld(42)
	q := ticker.NewQueue()
	h := &commandHandler{w: w, q: q}

	builder1 := w.UnitsByTeam(entity.Team1)[0]
	builder2 := w.UnitsByTeam(entity.Team1)[1]
	target1 := hex.Coord{Q: 6, R: 5}
	target2 := hex.Coord{Q: 6, R: 6}

	w.WriteFunc(func() {
		builder1.SetPosition(hex.Coord{Q: 5, R: 5})
		builder2.SetPosition(hex.Coord{Q: 5, R: 6})
		w.Tiles[target1] = terrain.Tile{Coord: target1, Terrain: terrain.Plain}
		w.Tiles[target2] = terrain.Tile{Coord: target2, Terrain: terrain.Plain}
	})

	first := map[string]any{
		"unit_id":       builder1.ID(),
		"kind":          "BUILD",
		"building_kind": "stable",
		"target_coord": map[string]any{
			"q": target1.Q,
			"r": target1.R,
		},
	}
	if rec := doCommandRequest(t, h, first, "1"); rec.Code != http.StatusAccepted {
		t.Fatalf("first build status = %d, want %d, body=%s", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	second := map[string]any{
		"unit_id":       builder2.ID(),
		"kind":          "BUILD",
		"building_kind": "stable",
		"target_coord": map[string]any{
			"q": target2.Q,
			"r": target2.R,
		},
	}
	rec := doCommandRequest(t, h, second, "1")
	assertErrorResponse(t, rec, http.StatusBadRequest, "insufficient_resources")
}

func TestCommandHandler_RejectsProduceWhenPopulationCapWouldBeExceeded(t *testing.T) {
	w := world.NewWorld(42)
	q := ticker.NewQueue()
	h := &commandHandler{w: w, q: q}

	var tc *entity.Building
	for _, b := range w.BuildingsByTeam(entity.Team1) {
		if b.Kind() == entity.KindTownCenter {
			tc = b
			break
		}
	}
	if tc == nil {
		t.Fatalf("missing town center")
	}
	for i := len(w.UnitsByTeam(entity.Team1)); i < entity.PopulationCap; i++ {
		w.SpawnUnit(entity.Team1, entity.KindInfantry, hex.Coord{Q: 10 + (i % 5), R: 5 + (i / 5)})
	}

	body := map[string]any{
		"building_id": tc.ID(),
		"kind":        "PRODUCE",
		"unit_kind":   "villager",
	}

	rec := doCommandRequest(t, h, body, "1")
	assertErrorResponse(t, rec, http.StatusBadRequest, "population_cap_reached")
}

func TestCommandsHandler_ReturnsPendingCommandsForTeamOnly(t *testing.T) {
	w := world.NewWorld(42)
	q := ticker.NewQueue()
	h := &commandsHandler{w: w, q: q}

	team1Unit := w.UnitsByTeam(entity.Team1)[0]
	team2Unit := w.UnitsByTeam(entity.Team2)[0]
	target := hex.Coord{Q: 7, R: 7}

	q.Submit(ticker.Command{
		Team:        entity.Team1,
		UnitID:      team1Unit.ID(),
		Kind:        ticker.CmdMoveFast,
		TargetCoord: &target,
	})
	q.Submit(ticker.Command{
		Team:   entity.Team2,
		UnitID: team2Unit.ID(),
		Kind:   ticker.CmdGather,
	})

	req := httptest.NewRequest(http.MethodGet, "/commands", nil)
	req.Header.Set("X-Team-ID", "1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Tick     uint64 `json:"tick"`
		Commands []struct {
			Team        entity.Team      `json:"team"`
			UnitID      *entity.EntityID `json:"unit_id,omitempty"`
			Kind        string           `json:"kind"`
			TargetCoord *struct {
				Q int `json:"q"`
				R int `json:"r"`
			} `json:"target_coord,omitempty"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal commands response: %v", err)
	}
	if resp.Tick != w.GetTick() {
		t.Fatalf("tick = %d, want %d", resp.Tick, w.GetTick())
	}
	if len(resp.Commands) != 1 {
		t.Fatalf("commands len = %d, want 1, body=%s", len(resp.Commands), rec.Body.String())
	}
	if resp.Commands[0].Team != entity.Team1 || resp.Commands[0].UnitID == nil || *resp.Commands[0].UnitID != team1Unit.ID() || resp.Commands[0].Kind != string(ticker.CmdMoveFast) {
		t.Fatalf("unexpected command: %+v", resp.Commands[0])
	}
	if resp.Commands[0].TargetCoord == nil || resp.Commands[0].TargetCoord.Q != target.Q || resp.Commands[0].TargetCoord.R != target.R {
		t.Fatalf("unexpected target coord: %+v", resp.Commands[0].TargetCoord)
	}
}

func TestCommandsHandler_ReflectsLastCommandWins(t *testing.T) {
	w := world.NewWorld(42)
	q := ticker.NewQueue()
	h := &commandsHandler{w: w, q: q}

	unit := w.UnitsByTeam(entity.Team1)[0]
	first := hex.Coord{Q: 6, R: 6}
	second := hex.Coord{Q: 8, R: 8}

	q.Submit(ticker.Command{
		Team:        entity.Team1,
		UnitID:      unit.ID(),
		Kind:        ticker.CmdMoveFast,
		TargetCoord: &first,
	})
	q.Submit(ticker.Command{
		Team:        entity.Team1,
		UnitID:      unit.ID(),
		Kind:        ticker.CmdMoveGuard,
		TargetCoord: &second,
	})

	req := httptest.NewRequest(http.MethodGet, "/commands", nil)
	req.Header.Set("X-Team-ID", "1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Commands []struct {
			Kind        string `json:"kind"`
			TargetCoord *struct {
				Q int `json:"q"`
				R int `json:"r"`
			} `json:"target_coord,omitempty"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal commands response: %v", err)
	}
	if len(resp.Commands) != 1 {
		t.Fatalf("commands len = %d, want 1, body=%s", len(resp.Commands), rec.Body.String())
	}
	if resp.Commands[0].Kind != string(ticker.CmdMoveGuard) {
		t.Fatalf("kind = %q, want %q", resp.Commands[0].Kind, ticker.CmdMoveGuard)
	}
	if resp.Commands[0].TargetCoord == nil || resp.Commands[0].TargetCoord.Q != second.Q || resp.Commands[0].TargetCoord.R != second.R {
		t.Fatalf("unexpected target coord: %+v", resp.Commands[0].TargetCoord)
	}
}

type commandErrorEnvelope struct {
	Error struct {
		Code   string `json:"code"`
		Reason string `json:"reason"`
	} `json:"error"`
}

func doCommandRequest(t *testing.T, h http.Handler, body map[string]any, team string) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/command", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Team-ID", team)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func assertErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, wantStatus, rec.Body.String())
	}
	var resp commandErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if resp.Error.Code != wantCode {
		t.Fatalf("error.code = %q, want %q", resp.Error.Code, wantCode)
	}
	if resp.Error.Reason == "" {
		t.Fatalf("error.reason should not be empty")
	}
}
