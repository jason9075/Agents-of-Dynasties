package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jason9075/agents_of_dynasties/internal/entity"
	"github.com/jason9075/agents_of_dynasties/internal/hex"
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
