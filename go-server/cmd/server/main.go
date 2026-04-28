// Command server is the single binary that hosts the admin panel, the
// kiosk SPA, the admin REST API, and the kiosk proxy. It also serves
// both built SPA bundles via http.FileServer with SPA-aware fallback,
// so we don't need nginx in front of it.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/maic/checkin-kiosk-api/internal/admin"
	"github.com/maic/checkin-kiosk-api/internal/auth"
	"github.com/maic/checkin-kiosk-api/internal/kiosk"
	"github.com/maic/checkin-kiosk-api/internal/proxy"
	"github.com/maic/checkin-kiosk-api/internal/store"
)

// settings is the (small) runtime configuration. Everything is env-driven.
type settings struct {
	port              string
	dataPath          string
	upstreamTimeoutMs int
	allowedOrigins    []string
	adminSPADir       string // path to the built admin SPA assets
	kioskSPADir       string // path to the built kiosk SPA assets
}

func loadSettings() settings {
	return settings{
		port:              getenv("PORT", "8089"),
		dataPath:          getenv("DATA_PATH", "data/data.db"),
		upstreamTimeoutMs: getenvInt("UPSTREAM_TIMEOUT_MS", 12000),
		allowedOrigins:    splitCSV(getenv("ALLOWED_ORIGINS", "")),
		adminSPADir:       getenv("ADMIN_SPA_DIR", "admin-spa-dist"),
		kioskSPADir:       getenv("KIOSK_SPA_DIR", "kiosk-spa-dist"),
	}
}

func main() {
	s := loadSettings()

	if dir := filepath.Dir(s.dataPath); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}

	st, err := store.Open(s.dataPath)
	if err != nil {
		log.Fatalf("store.Open: %v", err)
	}
	defer st.Close()
	log.Printf("opened SQLite at %s", s.dataPath)

	pr := proxy.New(time.Duration(s.upstreamTimeoutMs) * time.Millisecond)
	adminH := admin.New(st)
	kioskH := kiosk.New(st, pr)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   originsOrSelf(s.allowedOrigins),
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "X-Lookup-Method", "X-Kiosk-Language", "Accept-Language"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Auth middleware runs once for every request, but only `/api/admin/*`
	// routes actually require a user; the kiosk path is unauthenticated.
	r.Use(auth.Middleware(st))

	r.Route("/api/admin/v1", adminH.Mount)
	r.Route("/api/kiosk/v1", kioskH.Mount)

	// Admin SPA (priority): `/admin/*` always served by the admin bundle.
	mountSPA(r, "/admin", s.adminSPADir)

	// Kiosk SPA: any other URL — including `/k_<uuid>/...` — falls through
	// to the kiosk bundle's index.html (router reads slug from window.location.pathname).
	mountSPA(r, "/", s.kioskSPADir)

	srv := &http.Server{
		Addr:              ":" + s.port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	go func() {
		log.Printf("checkin-kiosk-api listening on :%s", s.port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

// mountSPA serves built SPA assets under `prefix` with SPA-aware
// fallback: any path that doesn't resolve to a real file falls back to
// the bundle's index.html so the client-side router can take over.
func mountSPA(r chi.Router, prefix, dir string) {
	if dir == "" {
		log.Printf("SPA dir for %s not set; skipping", prefix)
		return
	}
	if _, err := os.Stat(dir); err != nil {
		log.Printf("SPA dir %q not found; skipping mount of %s", dir, prefix)
		return
	}
	fs := http.FileServer(http.Dir(dir))
	indexPath := filepath.Join(dir, "index.html")
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// `path` is the slice of the URL inside the prefix.
		path := strings.TrimPrefix(req.URL.Path, prefix)
		if path == "" {
			path = "/"
		}
		if info, err := os.Stat(filepath.Join(dir, path)); err == nil && !info.IsDir() {
			http.StripPrefix(prefix, fs).ServeHTTP(w, req)
			return
		}
		http.ServeFile(w, req, indexPath)
	})
	if prefix == "/" {
		r.Handle("/*", handler)
		r.Handle("/", handler)
		return
	}
	r.Handle(prefix, handler)
	r.Handle(prefix+"/*", handler)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Printf("env %s must be int, got %q — using %d", key, v, fallback)
		return fallback
	}
	return n
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// originsOrSelf turns an empty allowlist into an explicit deny so the
// CORS layer doesn't accidentally accept everything when an operator
// forgets to set it.
func originsOrSelf(in []string) []string {
	if len(in) == 0 {
		return []string{"https://localhost"}
	}
	return in
}
