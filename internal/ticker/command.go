package ticker

import (
	"fmt"
	"sync"
	"time"

	"github.com/jason9075/agents_of_dynasties/internal/entity"
	"github.com/jason9075/agents_of_dynasties/internal/hex"
)

// CommandKind identifies the type of action an agent is requesting.
type CommandKind string

const (
	CmdMoveFast  CommandKind = "MOVE_FAST"
	CmdMoveGuard CommandKind = "MOVE_GUARD"
	CmdAttack    CommandKind = "ATTACK"
	CmdGather    CommandKind = "GATHER"
	CmdBuild     CommandKind = "BUILD"
	CmdProduce   CommandKind = "PRODUCE"
)

// Command is an action submitted by an agent for a specific unit.
type Command struct {
	Team         entity.Team      `json:"team"`
	UnitID       entity.EntityID  `json:"unit_id"`
	BuildingID   *entity.EntityID `json:"building_id,omitempty"`
	Kind         CommandKind      `json:"kind"`
	TargetCoord  *hex.Coord       `json:"target_coord,omitempty"`
	TargetID     *entity.EntityID `json:"target_id,omitempty"`
	BuildingKind *string          `json:"building_kind,omitempty"` // for BUILD
	UnitKind     *string          `json:"unit_kind,omitempty"`     // for PRODUCE
	ReceivedAt   time.Time        `json:"-"`
}

// Queue holds at most one pending command per unit (last-command-wins).
type Queue struct {
	mu      sync.Mutex
	pending map[string]Command
}

// NewQueue creates an empty command queue.
func NewQueue() *Queue {
	return &Queue{pending: make(map[string]Command)}
}

// Submit records cmd, replacing any prior command for the same unit.
func (q *Queue) Submit(cmd Command) {
	cmd.ReceivedAt = time.Now()
	q.mu.Lock()
	defer q.mu.Unlock()
	q.pending[commandActorKey(cmd)] = cmd
}

// Drain atomically removes and returns all pending commands.
func (q *Queue) Drain() []Command {
	q.mu.Lock()
	old := q.pending
	q.pending = make(map[string]Command, len(old))
	q.mu.Unlock()

	cmds := make([]Command, 0, len(old))
	for _, c := range old {
		cmds = append(cmds, c)
	}
	return cmds
}

func commandActorKey(cmd Command) string {
	if cmd.BuildingID != nil {
		return fmt.Sprintf("b:%d", *cmd.BuildingID)
	}
	return fmt.Sprintf("u:%d", cmd.UnitID)
}
