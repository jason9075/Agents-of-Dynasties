package api

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/jason9075/agents_of_dynasties/internal/entity"
	"github.com/jason9075/agents_of_dynasties/internal/hex"
	"github.com/jason9075/agents_of_dynasties/internal/terrain"
	"github.com/jason9075/agents_of_dynasties/internal/ticker"
	"github.com/jason9075/agents_of_dynasties/internal/world"
)

// --- Response shapes ---

type mapResponse struct {
	Width  int            `json:"width"`
	Height int            `json:"height"`
	Tiles  []terrain.Tile `json:"tiles"`
}

type unitView struct {
	ID       entity.EntityID `json:"id"`
	Kind     string          `json:"kind"`
	Team     entity.Team     `json:"team"`
	Position coordView       `json:"position"`
	HP       int             `json:"hp"`
	MaxHP    int             `json:"max_hp"`
	Friendly bool            `json:"friendly"`
}

type buildingView struct {
	ID       entity.EntityID `json:"id"`
	Kind     string          `json:"kind"`
	Team     entity.Team     `json:"team"`
	Position coordView       `json:"position"`
	HP       int             `json:"hp"`
	MaxHP    int             `json:"max_hp"`
	Friendly bool            `json:"friendly"`
}

type coordView struct {
	Q int `json:"q"`
	R int `json:"r"`
}

type stateResponse struct {
	Tick      uint64         `json:"tick"`
	Resources world.Resources `json:"resources"`
	Units     []unitView     `json:"units"`
	Buildings []buildingView `json:"buildings"`
}

// --- Map handler (cached after first call) ---

type mapHandler struct {
	w    *world.World
	once sync.Once
	data []byte
}

func (h *mapHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	h.once.Do(func() {
		resp := mapResponse{
			Width:  hex.GridWidth,
			Height: hex.GridHeight,
			Tiles:  h.w.AllTiles(),
		}
		h.data, _ = json.Marshal(resp)
	})
	w.Header().Set("Content-Type", "application/json")
	w.Write(h.data)
}

// --- State handler ---

type stateHandler struct {
	w *world.World
}

func (h *stateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	team, err := teamFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_team_header", err.Error())
		return
	}

	ownUnits, ownBuildings, enemyUnits, enemyBuildings := h.w.VisibleTo(team)

	var units []unitView
	for _, u := range ownUnits {
		pos := u.Position()
		units = append(units, unitView{
			ID:       u.ID(),
			Kind:     u.Kind().String(),
			Team:     u.Team(),
			Position: coordView{Q: pos.Q, R: pos.R},
			HP:       u.HP(),
			MaxHP:    u.MaxHP(),
			Friendly: true,
		})
	}
	for _, u := range enemyUnits {
		pos := u.Position()
		units = append(units, unitView{
			ID:       u.ID(),
			Kind:     u.Kind().String(),
			Team:     u.Team(),
			Position: coordView{Q: pos.Q, R: pos.R},
			HP:       u.HP(),
			MaxHP:    u.MaxHP(),
			Friendly: false,
		})
	}

	var buildings []buildingView
	for _, b := range ownBuildings {
		pos := b.Position()
		buildings = append(buildings, buildingView{
			ID:       b.ID(),
			Kind:     b.Kind().String(),
			Team:     b.Team(),
			Position: coordView{Q: pos.Q, R: pos.R},
			HP:       b.HP(),
			MaxHP:    b.MaxHP(),
			Friendly: true,
		})
	}
	for _, b := range enemyBuildings {
		pos := b.Position()
		buildings = append(buildings, buildingView{
			ID:       b.ID(),
			Kind:     b.Kind().String(),
			Team:     b.Team(),
			Position: coordView{Q: pos.Q, R: pos.R},
			HP:       b.HP(),
			MaxHP:    b.MaxHP(),
			Friendly: false,
		})
	}

	resp := stateResponse{
		Tick:      h.w.GetTick(),
		Resources: h.w.GetResources(team),
		Units:     units,
		Buildings: buildings,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// --- Full state handler (god-mode, no LOS masking) ---

type fullStateTeam struct {
	Resources world.Resources `json:"resources"`
	Units     []unitView      `json:"units"`
	Buildings []buildingView  `json:"buildings"`
}

type fullStateResponse struct {
	Tick  uint64        `json:"tick"`
	Team1 fullStateTeam `json:"team1"`
	Team2 fullStateTeam `json:"team2"`
}

type fullStateHandler struct {
	w *world.World
}

func (h *fullStateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	teamData := func(team entity.Team) fullStateTeam {
		var units []unitView
		for _, u := range h.w.UnitsByTeam(team) {
			pos := u.Position()
			units = append(units, unitView{
				ID:       u.ID(),
				Kind:     u.Kind().String(),
				Team:     u.Team(),
				Position: coordView{Q: pos.Q, R: pos.R},
				HP:       u.HP(),
				MaxHP:    u.MaxHP(),
				Friendly: true,
			})
		}
		var buildings []buildingView
		for _, b := range h.w.BuildingsByTeam(team) {
			pos := b.Position()
			buildings = append(buildings, buildingView{
				ID:       b.ID(),
				Kind:     b.Kind().String(),
				Team:     b.Team(),
				Position: coordView{Q: pos.Q, R: pos.R},
				HP:       b.HP(),
				MaxHP:    b.MaxHP(),
				Friendly: true,
			})
		}
		return fullStateTeam{
			Resources: h.w.GetResources(team),
			Units:     units,
			Buildings: buildings,
		}
	}

	resp := fullStateResponse{
		Tick:  h.w.GetTick(),
		Team1: teamData(entity.Team1),
		Team2: teamData(entity.Team2),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// --- Command handler ---

type commandHandler struct {
	w *world.World
	q *ticker.Queue
}

func (h *commandHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	team, err := teamFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_team_header", err.Error())
		return
	}

	var cmd ticker.Command
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON: "+err.Error())
		return
	}
	cmd.Team = team

	switch cmd.Kind {
	case ticker.CmdProduce:
		if cmd.BuildingID == nil {
			writeError(w, http.StatusBadRequest, "missing_building_id", "building_id is required for PRODUCE")
			return
		}
		building := h.w.GetBuilding(*cmd.BuildingID)
		if building == nil {
			writeError(w, http.StatusNotFound, "building_not_found", "building not found")
			return
		}
		if building.Team() != team {
			writeError(w, http.StatusForbidden, "building_wrong_team", "building does not belong to your team")
			return
		}
	default:
		unit := h.w.GetUnit(cmd.UnitID)
		if unit == nil {
			writeError(w, http.StatusNotFound, "unit_not_found", "unit not found")
			return
		}
		if unit.Team() != team {
			writeError(w, http.StatusForbidden, "unit_wrong_team", "unit does not belong to your team")
			return
		}
	}

	if status, code, reason := h.validateCommand(cmd); status != 0 {
		writeError(w, status, code, reason)
		return
	}

	h.q.Submit(cmd)
	w.WriteHeader(http.StatusAccepted)
}

func (h *commandHandler) validateCommand(cmd ticker.Command) (int, string, string) {
	switch cmd.Kind {
	case ticker.CmdMoveFast, ticker.CmdMoveGuard:
		return h.validateMove(cmd)
	case ticker.CmdGather:
		return h.validateGather(cmd)
	case ticker.CmdBuild:
		return h.validateBuild(cmd)
	case ticker.CmdAttack:
		return h.validateAttack(cmd)
	case ticker.CmdProduce:
		return h.validateProduce(cmd)
	default:
		return http.StatusBadRequest, "invalid_command_kind", "unsupported command kind"
	}
}

func (h *commandHandler) validateMove(cmd ticker.Command) (int, string, string) {
	if cmd.TargetCoord == nil {
		return http.StatusBadRequest, "missing_target_coord", "target_coord is required for MOVE commands"
	}
	if !hex.InBounds(*cmd.TargetCoord) {
		return http.StatusBadRequest, "target_out_of_bounds", "target_coord is outside the map"
	}
	tile, ok := h.w.Tile(*cmd.TargetCoord)
	if !ok || !tile.Terrain.Passable() {
		return http.StatusBadRequest, "target_not_passable", "target_coord is not passable"
	}
	return 0, "", ""
}

func (h *commandHandler) validateGather(cmd ticker.Command) (int, string, string) {
	unit := h.w.GetUnit(cmd.UnitID)
	if unit.Kind() != entity.KindVillager {
		return http.StatusBadRequest, "unit_cannot_gather", "only villagers can gather resources"
	}
	tile, ok := h.w.Tile(unit.Position())
	if !ok || tile.Terrain.ResourceYield() == terrain.ResourceNone {
		return http.StatusBadRequest, "no_gatherable_resource", "unit is not standing on a gatherable resource tile"
	}
	return 0, "", ""
}

func (h *commandHandler) validateBuild(cmd ticker.Command) (int, string, string) {
	if cmd.TargetCoord == nil {
		return http.StatusBadRequest, "missing_target_coord", "target_coord is required for BUILD"
	}
	if cmd.BuildingKind == nil {
		return http.StatusBadRequest, "missing_building_kind", "building_kind is required for BUILD"
	}
	builder := h.w.GetUnit(cmd.UnitID)
	if builder.Kind() != entity.KindVillager {
		return http.StatusBadRequest, "unit_cannot_build", "only villagers can construct buildings"
	}
	kind, ok := entity.ParseBuildingKind(*cmd.BuildingKind)
	if !ok {
		return http.StatusBadRequest, "invalid_building_kind", "unknown building_kind"
	}
	if kind == entity.KindTownCenter {
		return http.StatusBadRequest, "building_not_allowed", "town_center cannot be built by villagers"
	}
	if !hex.InBounds(*cmd.TargetCoord) {
		return http.StatusBadRequest, "target_out_of_bounds", "target_coord is outside the map"
	}
	if hex.Distance(builder.Position(), *cmd.TargetCoord) > 1 {
		return http.StatusBadRequest, "target_out_of_range", "builder must be adjacent to the build target"
	}
	tile, ok := h.w.Tile(*cmd.TargetCoord)
	if !ok || tile.Terrain != terrain.Plain {
		return http.StatusBadRequest, "invalid_build_tile", "build target must be a plain tile"
	}
	if !h.w.CanOccupy(*cmd.TargetCoord) {
		return http.StatusBadRequest, "target_occupied", "build target is occupied"
	}
	cost, ok := entity.BuildingCosts[kind]
	if !ok {
		return http.StatusBadRequest, "invalid_building_kind", "unknown building_kind"
	}
	if !h.w.CanAfford(cmd.Team, cost) {
		return http.StatusBadRequest, "insufficient_resources", "team cannot afford this building"
	}
	return 0, "", ""
}

func (h *commandHandler) validateAttack(cmd ticker.Command) (int, string, string) {
	if cmd.TargetID == nil {
		return http.StatusBadRequest, "missing_target_id", "target_id is required for ATTACK"
	}
	attacker := h.w.GetUnit(cmd.UnitID)
	if targetUnit := h.w.GetUnit(*cmd.TargetID); targetUnit != nil {
		if targetUnit.Team() == attacker.Team() {
			return http.StatusBadRequest, "friendly_fire_forbidden", "cannot attack a friendly unit"
		}
		if hex.Distance(attacker.Position(), targetUnit.Position()) > entity.AttackRange(attacker.Kind()) {
			return http.StatusBadRequest, "target_out_of_range", "target is outside attack range"
		}
		return 0, "", ""
	}
	if targetBuilding := h.w.GetBuilding(*cmd.TargetID); targetBuilding != nil {
		if targetBuilding.Team() == attacker.Team() {
			return http.StatusBadRequest, "friendly_fire_forbidden", "cannot attack a friendly building"
		}
		if hex.Distance(attacker.Position(), targetBuilding.Position()) > entity.AttackRange(attacker.Kind()) {
			return http.StatusBadRequest, "target_out_of_range", "target is outside attack range"
		}
		return 0, "", ""
	}
	return http.StatusNotFound, "target_not_found", "attack target does not exist"
}

func (h *commandHandler) validateProduce(cmd ticker.Command) (int, string, string) {
	if cmd.UnitKind == nil {
		return http.StatusBadRequest, "missing_unit_kind", "unit_kind is required for PRODUCE"
	}
	building := h.w.GetBuilding(*cmd.BuildingID)
	kind, ok := entity.ParseUnitKind(*cmd.UnitKind)
	if !ok {
		return http.StatusBadRequest, "invalid_unit_kind", "unknown unit_kind"
	}
	if entity.UnitProducer(kind) != building.Kind() {
		return http.StatusBadRequest, "invalid_producer", "this building cannot produce the requested unit kind"
	}
	cost, ok := entity.UnitCosts[kind]
	if !ok {
		return http.StatusBadRequest, "invalid_unit_kind", "unknown unit_kind"
	}
	if !h.w.CanAfford(cmd.Team, cost) {
		return http.StatusBadRequest, "insufficient_resources", "team cannot afford this unit"
	}
	return 0, "", ""
}
