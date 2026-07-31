package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type apiHandler func(http.ResponseWriter, *http.Request) error

type Server struct {
	cfg      Config
	db       *sql.DB
	realtime *Realtime
	handler  http.Handler
}

func New(ctx context.Context, cfg Config) (*Server, error) {
	db, err := openDatabase(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if cfg.SeedOnBoot {
		if err := seedIfEmpty(ctx, db, cfg); err != nil {
			db.Close()
			return nil, fmt.Errorf("seed database: %w", err)
		}
	}
	server := &Server{cfg: cfg, db: db, realtime: newRealtime(db, cfg)}
	server.handler = server.routes()
	return server, nil
}

func (s *Server) Handler() http.Handler { return s.handler }

func (s *Server) Close() error {
	s.realtime.Close()
	return s.db.Close()
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/socket.io/", s.realtime.Handler())

	s.route(mux, "GET /api", s.apiRoot)
	s.route(mux, "GET /api/{$}", s.apiRoot)
	s.route(mux, "GET /api/health", s.health)
	s.route(mux, "POST /api/auth/login", s.login)
	s.route(mux, "GET /api/auth/me", s.authenticate(s.me))
	s.route(mux, "POST /api/auth/change-password", s.authenticate(s.changePassword))

	s.route(mux, "GET /api/users", s.authenticate(s.listUsers))
	s.route(mux, "POST /api/users", s.authenticate(s.createUser, RoleManager))
	s.route(mux, "PATCH /api/users/{id}", s.authenticate(s.updateUser, RoleManager))
	s.route(mux, "DELETE /api/users/{id}", s.authenticate(s.deleteUser, RoleManager))

	s.route(mux, "GET /api/stores/current", s.authenticate(s.currentStore))

	s.route(mux, "GET /api/shifts", s.authenticate(s.listShifts))
	s.route(mux, "POST /api/shifts", s.authenticate(s.createShift, RoleManager))
	s.route(mux, "PATCH /api/shifts/{id}", s.authenticate(s.updateShift, RoleManager))
	s.route(mux, "DELETE /api/shifts/{id}", s.authenticate(s.deleteShift, RoleManager))

	s.route(mux, "GET /api/schedules", s.authenticate(s.getMonthSchedules))
	s.route(mux, "GET /api/schedules/storage", s.authenticate(s.scheduleStorage, RoleManager))
	s.route(mux, "PUT /api/schedules/month", s.authenticate(s.saveMonthSchedules, RoleManager))
	s.route(mux, "POST /api/schedules/cleanup", s.authenticate(s.cleanupSchedules, RoleManager))

	s.route(mux, "GET /api/requests", s.authenticate(s.listRequests))
	s.route(mux, "POST /api/requests", s.authenticate(s.createRequest))
	s.route(mux, "POST /api/requests/clear-resolved", s.authenticate(s.clearResolvedRequests))
	s.route(mux, "DELETE /api/requests/{id}", s.authenticate(s.deleteRequest))
	s.route(mux, "POST /api/requests/{id}/approve", s.authenticate(s.approveRequest, RoleManager))
	s.route(mux, "POST /api/requests/{id}/reject", s.authenticate(s.rejectRequest, RoleManager))

	s.route(mux, "GET /api/notifications", s.authenticate(s.listNotifications))
	s.route(mux, "POST /api/notifications/read-all", s.authenticate(s.markAllNotificationsRead))
	s.route(mux, "POST /api/notifications/clear", s.authenticate(s.clearNotifications))
	s.route(mux, "POST /api/notifications/{id}/read", s.authenticate(s.markNotificationRead))
	s.route(mux, "DELETE /api/notifications/{id}", s.authenticate(s.deleteNotification))

	mux.HandleFunc("/", s.serveFrontend)
	return s.securityHeadersMiddleware(s.recoverMiddleware(s.corsMiddleware(mux)))
}

func (s *Server) securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) route(mux *http.ServeMux, pattern string, handler apiHandler) {
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if err := handler(w, r); err != nil {
			var apiErr *apiError
			if !errors.As(err, &apiErr) {
				log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
			}
			writeError(w, err)
		}
	})
}

func (s *Server) apiRoot(w http.ResponseWriter, _ *http.Request) error {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"service": "shift-schedule-api",
		"message": "排班系统后端运行正常",
		"docs": map[string]string{
			"health":        "GET /api/health",
			"login":         "POST /api/auth/login",
			"me":            "GET /api/auth/me",
			"schedules":     "GET /api/schedules?month=YYYY-MM",
			"requests":      "GET /api/requests",
			"notifications": "GET /api/notifications",
		},
		"clients": map[string]string{
			"web":         "同端口静态页面（生产）或 http://localhost:5173（开发）",
			"miniprogram": "apps/miniprogram (微信开发者工具打开)",
		},
		"frontend": "生产环境由本服务托管前端静态资源",
		"time":     time.Now().UTC().Format(time.RFC3339Nano),
	})
	return nil
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.db.PingContext(ctx); err != nil {
		return serviceUnavailable("数据库暂不可用")
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "shift-schedule-api", "time": time.Now().UTC().Format(time.RFC3339Nano)})
	return nil
}

func (s *Server) currentStore(w http.ResponseWriter, r *http.Request) error {
	var store Store
	var address sql.NullString
	err := s.db.QueryRowContext(r.Context(), "SELECT `id`, `name`, `address`, `createdAt`, `updatedAt` FROM `stores` WHERE `id` = ? LIMIT 1", requestClaims(r).StoreID).Scan(&store.ID, &store.Name, &address, &store.CreatedAt, &store.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusOK, nil)
		return nil
	}
	if err != nil {
		return err
	}
	store.Address = ptrFromNull(address)
	writeJSON(w, http.StatusOK, store)
	return nil
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && s.originAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) originAllowed(origin string) bool {
	for _, allowed := range s.cfg.CORSOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}

func (s *Server) recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("panic serving %s %s: %v", r.Method, r.URL.Path, recovered)
				writeError(w, errors.New("unexpected panic"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) serveFrontend(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api") || strings.HasPrefix(r.URL.Path, "/socket.io") {
		writeError(w, notFound("Cannot "+r.Method+" "+r.URL.Path))
		return
	}
	if s.cfg.WebDist == "" || r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}
	path := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	if path == "." || !strings.Contains(filepath.Base(path), ".") {
		path = "index.html"
	}
	filename := filepath.Join(s.cfg.WebDist, path)
	relative, err := filepath.Rel(s.cfg.WebDist, filename)
	if err != nil || strings.HasPrefix(relative, "..") {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(filename)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	if contentType := mime.TypeByExtension(filepath.Ext(filename)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	if filepath.Base(filename) != "index.html" {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	http.ServeFile(w, r, filename)
}
