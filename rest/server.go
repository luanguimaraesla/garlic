package rest

import (
	"context"
	"net/http"
	"time"

	chi "github.com/go-chi/chi/v5"

	"github.com/luanguimaraesla/garlic/errors"
	"github.com/luanguimaraesla/garlic/logging"
)

const defaultShutdownTimeout = 30 * time.Second

var servers map[string]*Server

// ServerOption configures a Server.
type ServerOption func(*Server)

// WithShutdownTimeout sets the maximum duration to wait for in-flight
// requests to complete during graceful shutdown. Defaults to 30s.
func WithShutdownTimeout(d time.Duration) ServerOption {
	return func(s *Server) {
		s.shutdownTimeout = d
	}
}

// WithOnShutdown registers a hook that is called when the server begins
// shutting down. The provided context carries the shutdown deadline.
func WithOnShutdown(fn func(context.Context)) ServerOption {
	return func(s *Server) {
		s.onShutdown = fn
	}
}

type Server struct {
	Name            string
	root            chi.Router
	shutdownTimeout time.Duration
	onShutdown      func(context.Context)
}

func NewServer(name string, opts ...ServerOption) *Server {
	router := chi.NewRouter()

	s := &Server{
		Name:            name,
		root:            router,
		shutdownTimeout: defaultShutdownTimeout,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *Server) Router() chi.Router {
	return s.root
}

// Listen starts an HTTP server on the specified bind address. When the
// context is cancelled, it performs a graceful shutdown, waiting for
// in-flight requests to complete within the configured timeout.
func (s *Server) Listen(ctx context.Context, bind string) <-chan error {
	l := logging.Global()
	errCh := make(chan error, 1)

	ectx := errors.Context(
		errors.Field("bind", bind),
		errors.Field("server", s.Name),
	)

	srv := &http.Server{
		Addr:    bind,
		Handler: s.Router(),
	}

	go func() {
		defer close(errCh)

		go func() {
			l.Info("Listening.", ectx.Zap())
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				errCh <- errors.PropagateAs(errors.KindSystemError, err, "failed to start HTTP server", ectx)
			}
		}()

		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
		defer cancel()

		if s.onShutdown != nil {
			s.onShutdown(shutdownCtx)
		}

		if err := srv.Shutdown(shutdownCtx); err != nil {
			errCh <- errors.PropagateAs(errors.KindSystemError, err, "failed to gracefully shutdown HTTP server", ectx)
		}
	}()

	return errCh
}

// GetServer implements a multiton of servers. Options are applied only
// when the server is created for the first time.
func GetServer(name string, opts ...ServerOption) *Server {
	if servers == nil {
		servers = make(map[string]*Server)
	}

	if srv, exists := servers[name]; exists {
		return srv
	}

	srv := NewServer(name, opts...)
	servers[name] = srv
	return srv
}
