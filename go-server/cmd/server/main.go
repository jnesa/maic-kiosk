// Command server is the kiosk Go binary. It is a stateless HTTP shim:
// chi router + cookie middleware + two static SPA bundles + the
// legacymaichttp adapter. No SQLite, no DB driver, no internal store.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
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

	"github.com/maic/checkin-kiosk-api/internal/adapters/legacymaichttp"
	"github.com/maic/checkin-kiosk-api/internal/admin"
	"github.com/maic/checkin-kiosk-api/internal/kiosk"
	"github.com/maic/checkin-kiosk-api/internal/session"
)

// settings is the (small) runtime configuration. Everything is env-driven.
type settings struct {
	port              string
	legacyBaseURL     string
	upstreamTimeoutMs int
	allowedOrigins    []string
	adminSPADir       string
	kioskSPADir       string
	sessionSecret     string
}

func loadSettings() settings {
	return settings{
		port:              getenv("PORT", "8089"),
		legacyBaseURL:     getenv("LEGACY_BASE_URL", "https://dev.maiccube.com"),
		upstreamTimeoutMs: getenvInt("UPSTREAM_TIMEOUT_MS", 15000),
		allowedOrigins:    splitCSV(getenv("ALLOWED_ORIGINS", "")),
		adminSPADir:       getenv("ADMIN_SPA_DIR", "admin-spa-dist"),
		kioskSPADir:       getenv("KIOSK_SPA_DIR", "kiosk-spa-dist"),
		sessionSecret:     getenv("KIOSK_SESSION_SECRET", ""),
	}
}

func main() {
	s := loadSettings()

	if s.sessionSecret == "" {
		s.sessionSecret = randomSecret()
		log.Printf("KIOSK_SESSION_SECRET not set — generated a random one for this process. " +
			"All operator sessions will be invalidated on restart. Set the env in production.")
	}

	// One adapter (legacymaichttp) implements every port the
	// handlers depend on. Future phases swap this for a different
	// adapter without touching the handlers.
	upstream := legacymaichttp.New(s.legacyBaseURL, time.Duration(s.upstreamTimeoutMs)*time.Millisecond)

	sm := session.NewManager(s.sessionSecret)
	adminH := admin.New(upstream, upstream, sm)
	kioskH := kiosk.New(upstream, upstream, upstream)

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

	// Cookie-based session middleware runs on every request; only
	// admin routes wrap RequireOperator on top.
	r.Use(sm.Middleware)

	r.Route("/api/admin/v1", adminH.Mount)
	r.Route("/api/kiosk/v1", kioskH.Mount)

	// Admin SPA at /admin/*; kiosk SPA at everything else
	// (including /k_<uuid>/...). Both fall through to index.html
	// for SPA-side routing.
	mountSPA(r, "/admin", s.adminSPADir)
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
		log.Printf("checkin-kiosk-api listening on :%s — legacy upstream: %s", s.port, s.legacyBaseURL)
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
// fallback: any path that doesn't resolve to a real file falls back
// to the bundle's index.html so the client-side router takes over.
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

// originsOrSelf turns an empty allowlist into an explicit deny so
// the CORS layer doesn't accidentally accept everything when an
// operator forgets to set it.
func originsOrSelf(in []string) []string {
	if len(in) == 0 {
		return []string{"https://localhost"}
	}
	return in
}

// randomSecret generates a 32-byte secret (base64) for the cookie
// signer when KIOSK_SESSION_SECRET is unset. Process-local; restarts
// invalidate cookies.
func randomSecret() string {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		log.Fatalf("rand.Read: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf[:])
}
