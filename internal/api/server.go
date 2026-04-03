package api

import (
	"net/http"

	"github.com/jason9075/agents_of_empires/internal/ticker"
	"github.com/jason9075/agents_of_empires/internal/world"
)

// Server is the HTTP API server for the game.
type Server struct {
	mux *http.ServeMux
}

// NewServer wires up all routes and returns a ready-to-serve Server.
func NewServer(w *world.World, q *ticker.Queue) *Server {
	mux := http.NewServeMux()

	mux.Handle("/map", &mapHandler{w: w})
	mux.Handle("/state", &stateHandler{w: w})
	mux.Handle("/command", &commandHandler{w: w, q: q})

	return &Server{mux: mux}
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
