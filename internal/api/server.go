package api

import (
	"context"
	"net/http"

	"github.com/openlibrecommunity/olcrtc/internal/channel"
)

// Server is the HTTP API server for channel management.
type Server struct {
	manager    *channel.Manager
	httpServer *http.Server
}

// NewServer creates a new API server.
func NewServer(mgr *channel.Manager, masterKey, listenAddr string) *Server {
	s := &Server{manager: mgr}

	mux := http.NewServeMux()

	// Channel CRUD.
	mux.HandleFunc("POST /api/v1/channels", s.handleCreateChannel)
	mux.HandleFunc("GET /api/v1/channels", s.handleListChannels)
	mux.HandleFunc("GET /api/v1/channels/{id}", s.handleGetChannel)
	mux.HandleFunc("PUT /api/v1/channels/{id}", s.handleUpdateChannel)
	mux.HandleFunc("DELETE /api/v1/channels/{id}", s.handleDeleteChannel)

	// Channel operations.
	mux.HandleFunc("POST /api/v1/channels/{id}/renew", s.handleRenewChannel)

	// Server status.
	mux.HandleFunc("GET /api/v1/status", s.handleStatus)

	s.httpServer = &http.Server{
		Addr:    listenAddr,
		Handler: authMiddleware(masterKey, mux),
	}

	return s
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
