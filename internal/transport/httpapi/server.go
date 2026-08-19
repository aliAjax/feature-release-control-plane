package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/example/feature-release-control-plane/internal/application"
	"github.com/example/feature-release-control-plane/internal/configdomain"
	"github.com/example/feature-release-control-plane/internal/platform/httpx"
	"github.com/example/feature-release-control-plane/internal/releasedomain"
)

type Server struct {
	service   *application.Service
	server    *http.Server
	started   atomic.Bool
	heartbeat time.Duration
}

func New(address string, service *application.Service, heartbeatSeconds int) *Server {
	if heartbeatSeconds < 1 {
		heartbeatSeconds = 15
	}
	s := &Server{service: service, heartbeat: time.Duration(heartbeatSeconds) * time.Second}
	mux := http.NewServeMux()
	s.routes(mux)
	s.server = &http.Server{Addr: address, Handler: logging(httpx.WithRequestID(mux)), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 0, IdleTimeout: 60 * time.Second}
	return s
}
func (s *Server) ListenAndServe() error {
	s.started.Store(true)
	err := s.server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
func (s *Server) Shutdown(ctx context.Context) error { return s.server.Shutdown(ctx) }
func (s *Server) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if !s.started.Load() {
			httpx.JSON(w, http.StatusServiceUnavailable, map[string]any{"status": "starting"})
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"status": "ready"})
	})
	mux.Handle("GET /debug/pprof/", http.DefaultServeMux)
	mux.Handle("POST /api/v1/namespaces", httpx.Adapt(s.createNamespace))
	mux.Handle("GET /api/v1/namespaces", httpx.Adapt(s.listNamespaces))
	mux.Handle("POST /api/v1/configs", httpx.Adapt(s.createConfig))
	mux.Handle("GET /api/v1/configs", httpx.Adapt(s.listConfigs))
	mux.Handle("POST /api/v1/configs/{id}/versions", httpx.Adapt(s.createVersion))
	mux.Handle("GET /api/v1/configs/{id}/versions", httpx.Adapt(s.listVersions))
	mux.Handle("POST /api/v1/configs/{id}/versions/{version}/validate", httpx.Adapt(s.validateVersion))
	mux.Handle("POST /api/v1/configs/{id}/versions/{version}/explain", httpx.Adapt(s.explainVersion))
	mux.Handle("POST /api/v1/configs/{id}/versions/{version}/diff", httpx.Adapt(s.diff))
	mux.Handle("POST /api/v1/configs/{id}/versions/{version}/approve", httpx.Adapt(s.approve))
	mux.Handle("POST /api/v1/releases", httpx.Adapt(s.createRelease))
	mux.Handle("GET /api/v1/releases", httpx.Adapt(s.listReleases))
	mux.Handle("POST /api/v1/releases/{id}/pause", httpx.Adapt(s.pause))
	mux.Handle("POST /api/v1/releases/{id}/resume", httpx.Adapt(s.resume))
	mux.Handle("POST /api/v1/releases/{id}/rollback", httpx.Adapt(s.rollback))
	mux.Handle("POST /api/v1/releases/{id}/health", httpx.Adapt(s.healthGate))
	mux.Handle("POST /api/v1/evaluate", httpx.Adapt(s.evaluate))
	mux.Handle("POST /api/v1/evaluate/batch", httpx.Adapt(s.evaluateBatch))
	mux.Handle("GET /api/v1/audit", httpx.Adapt(s.queryAudit))
	mux.Handle("GET /api/v1/stream", http.HandlerFunc(s.stream))
}
func actor(r *http.Request) string {
	v := strings.TrimSpace(r.Header.Get("X-Actor-ID"))
	if v == "" {
		return "anonymous"
	}
	return v
}
func scope(r *http.Request) (configdomain.Scope, error) {
	q := r.URL.Query()
	s := configdomain.Scope{TenantID: q.Get("tenant_id"), Application: q.Get("application"), Environment: q.Get("environment"), Namespace: q.Get("namespace")}
	return s, s.Validate()
}
func (s *Server) createNamespace(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Scope configdomain.Scope `json:"scope"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return fmt.Errorf("%w: %v", httpx.ErrBadRequest, err)
	}
	if err := in.Scope.Validate(); err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"scope": in.Scope, "message": "namespace is logical and created on first configuration"})
	return nil
}
func (s *Server) listNamespaces(w http.ResponseWriter, r *http.Request) error {
	httpx.JSON(w, http.StatusOK, map[string]any{"items": []any{}, "message": "namespaces are discovered from persisted configuration scopes"})
	return nil
}
func (s *Server) createConfig(w http.ResponseWriter, r *http.Request) error {
	var in application.CreateConfig
	if err := httpx.Decode(r, &in); err != nil {
		return fmt.Errorf("%w: %v", httpx.ErrBadRequest, err)
	}
	c, err := s.service.CreateConfig(r.Context(), actor(r), in)
	if err != nil {
		return err
	}
	w.Header().Set("ETag", httpx.ETag(c.Version))
	httpx.JSON(w, http.StatusCreated, c)
	return nil
}
func (s *Server) listConfigs(w http.ResponseWriter, r *http.Request) error {
	sc, err := scope(r)
	if err != nil {
		return err
	}
	page, size := httpx.Page(r)
	items, total, err := s.service.ListConfigs(r.Context(), sc, page, size)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items, "page": page, "page_size": size, "total": total})
	return nil
}
func (s *Server) createVersion(w http.ResponseWriter, r *http.Request) error {
	var in application.CreateVersion
	if err := httpx.Decode(r, &in); err != nil {
		return fmt.Errorf("%w: %v", httpx.ErrBadRequest, err)
	}
	v, err := s.service.CreateVersion(r.Context(), actor(r), r.PathValue("id"), in)
	if err != nil {
		return err
	}
	w.Header().Set("ETag", httpx.ETag(v.Revision))
	httpx.JSON(w, http.StatusCreated, v)
	return nil
}
func (s *Server) listVersions(w http.ResponseWriter, r *http.Request) error {
	items, err := s.service.ListVersions(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
	return nil
}
func (s *Server) validateVersion(w http.ResponseWriter, r *http.Request) error {
	num, err := version(r)
	if err != nil {
		return err
	}
	report, err := s.service.ValidateVersion(r.Context(), r.PathValue("id"), num)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, report)
	return nil
}
func (s *Server) explainVersion(w http.ResponseWriter, r *http.Request) error {
	num, err := version(r)
	if err != nil {
		return err
	}
	var in struct {
		Subject configdomain.Subject `json:"subject"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return fmt.Errorf("%w: %v", httpx.ErrBadRequest, err)
	}
	out, err := s.service.ExplainVersion(r.Context(), r.PathValue("id"), num, in.Subject)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, out)
	return nil
}
func (s *Server) diff(w http.ResponseWriter, r *http.Request) error {
	to, err := version(r)
	if err != nil {
		return err
	}
	var in struct {
		From int64 `json:"from"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return fmt.Errorf("%w: %v", httpx.ErrBadRequest, err)
	}
	if in.From < 1 {
		return fmt.Errorf("%w: from must be positive", httpx.ErrBadRequest)
	}
	out, err := s.service.Diff(r.Context(), r.PathValue("id"), in.From, to)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
	return nil
}
func (s *Server) approve(w http.ResponseWriter, r *http.Request) error {
	num, err := version(r)
	if err != nil {
		return err
	}
	var in struct {
		Revision int64 `json:"revision"`
		Schedule bool  `json:"schedule"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return fmt.Errorf("%w: %v", httpx.ErrBadRequest, err)
	}
	to := configdomain.Reviewing
	if in.Schedule {
		to = configdomain.Scheduled
	}
	v, err := s.service.TransitionVersion(r.Context(), actor(r), r.PathValue("id"), num, to, in.Revision)
	if err != nil {
		return err
	}
	w.Header().Set("ETag", httpx.ETag(v.Revision))
	httpx.JSON(w, http.StatusOK, v)
	return nil
}
func version(r *http.Request) (int64, error) {
	n, err := strconv.ParseInt(r.PathValue("version"), 10, 64)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%w: invalid version", httpx.ErrBadRequest)
	}
	return n, nil
}
func (s *Server) createRelease(w http.ResponseWriter, r *http.Request) error {
	var in releasedomain.Release
	if err := httpx.Decode(r, &in); err != nil {
		return fmt.Errorf("%w: %v", httpx.ErrBadRequest, err)
	}
	in.IdempotencyKey = r.Header.Get("Idempotency-Key")
	out, err := s.service.CreateRelease(r.Context(), actor(r), in)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, out)
	return nil
}
func (s *Server) listReleases(w http.ResponseWriter, r *http.Request) error {
	page, size := httpx.Page(r)
	items, total, err := s.service.ListReleases(r.Context(), page, size)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items, "page": page, "page_size": size, "total": total})
	return nil
}
func (s *Server) pause(w http.ResponseWriter, r *http.Request) error {
	return s.releaseTransition(w, r, releasedomain.Paused)
}
func (s *Server) resume(w http.ResponseWriter, r *http.Request) error {
	return s.releaseTransition(w, r, releasedomain.Running)
}
func (s *Server) rollback(w http.ResponseWriter, r *http.Request) error {
	return s.releaseTransition(w, r, releasedomain.RolledBack)
}
func (s *Server) healthGate(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		releasedomain.HealthGate
		FencingToken int64  `json:"fencing_token"`
		Reason       string `json:"reason"`
		Apply        bool   `json:"apply"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return fmt.Errorf("%w: %v", httpx.ErrBadRequest, err)
	}
	if !in.Apply {
		assessment, err := s.service.AssessReleaseHealth(r.Context(), r.PathValue("id"), in.HealthGate)
		if err != nil {
			return err
		}
		httpx.JSON(w, http.StatusOK, assessment)
		return nil
	}
	assessment, release, err := s.service.ApplyHealthGate(r.Context(), actor(r), r.PathValue("id"), in.HealthGate, in.FencingToken, in.Reason)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"assessment": assessment, "release": release})
	return nil
}
func (s *Server) releaseTransition(w http.ResponseWriter, r *http.Request, to releasedomain.State) error {
	var in struct {
		Reason       string `json:"reason"`
		FencingToken int64  `json:"fencing_token"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return fmt.Errorf("%w: %v", httpx.ErrBadRequest, err)
	}
	out, err := s.service.TransitionRelease(r.Context(), actor(r), r.PathValue("id"), to, in.Reason, in.FencingToken)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, out)
	return nil
}
func (s *Server) evaluate(w http.ResponseWriter, r *http.Request) error {
	var in application.EvaluateRequest
	if err := httpx.Decode(r, &in); err != nil {
		return fmt.Errorf("%w: %v", httpx.ErrBadRequest, err)
	}
	in.KnownETag = r.Header.Get("If-None-Match")
	out, err := s.service.Evaluate(r.Context(), in)
	if err != nil {
		return err
	}
	w.Header().Set("ETag", out.ETag)
	if out.NotModified {
		w.WriteHeader(http.StatusNotModified)
		return nil
	}
	httpx.JSON(w, http.StatusOK, out)
	return nil
}
func (s *Server) evaluateBatch(w http.ResponseWriter, r *http.Request) error {
	var in application.BatchEvaluateRequest
	if err := httpx.Decode(r, &in); err != nil {
		return fmt.Errorf("%w: %v", httpx.ErrBadRequest, err)
	}
	out, err := s.service.EvaluateBatch(r.Context(), in)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) queryAudit(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	query := application.AuditQuery{Scope: q.Get("scope"), Actor: q.Get("actor"), Action: q.Get("action"), Resource: q.Get("resource")}
	var err error
	if value := q.Get("from"); value != "" {
		var parsed time.Time
		parsed, err = time.Parse(time.RFC3339, value)
		if err != nil {
			return fmt.Errorf("%w: invalid from timestamp", httpx.ErrBadRequest)
		}
		query.From = &parsed
	}
	if value := q.Get("to"); value != "" {
		var parsed time.Time
		parsed, err = time.Parse(time.RFC3339, value)
		if err != nil {
			return fmt.Errorf("%w: invalid to timestamp", httpx.ErrBadRequest)
		}
		query.To = &parsed
	}
	page, size := httpx.Page(r)
	out, err := s.service.QueryAudit(r.Context(), query, page, size)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, out)
	return nil
}
func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("namespace")
	flusher, ok := w.(http.Flusher)
	if !ok {
		httpx.WriteError(w, r, fmt.Errorf("%w: streaming unsupported", httpx.ErrPrecondition))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch, cancel := s.service.Subscribe()
	defer cancel()
	ticker := time.NewTicker(s.heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		case event := <-ch:
			if scope != "" && !strings.Contains(event.Scope, scope) {
				continue
			}
			b, _ := json.Marshal(event)
			fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.Cursor, event.Type, b)
			flusher.Flush()
		}
	}
}
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("http request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(started).Milliseconds(), "request_id", httpx.RequestID(r))
	})
}
