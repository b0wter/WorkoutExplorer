// Package httpapi is the HTTP adapter of the Go server: it maps /api/*
// requests to application operations. It serves the health endpoint
// (architect/F-01-project-skeleton-and-ci.md, D-3) and, since F-09, the
// upload endpoint POST /api/workouts.
package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/netip"
	"time"
)

// healthResponse is the body of GET /api/health. The shape is a contract
// with the uptime check (F-31) and the placeholder page.
type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// StorageGate tells whether new uploads are accepted (UP-8, F-10);
// *lifecycle.Storage satisfies it.
type StorageGate interface {
	UploadsOpen() bool
}

// statusResponse is the body of GET /api/status (F-10 design): uploads_open
// and nothing else. A contract with the upload page (F-11).
type statusResponse struct {
	UploadsOpen bool `json:"uploads_open"`
}

// statusHandler answers GET and HEAD with the current gate state. It has no
// side effects and reveals nothing but the one boolean.
func statusHandler(g StorageGate) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, statusResponse{UploadsOpen: g.UploadsOpen()})
	})
}

// Config is everything NewHandler needs.
type Config struct {
	Version              string
	Uploads              Uploader         // nil: POST /api/workouts is not served
	Owner                Owner            // nil: the owner endpoints (F-12) are not served
	Reports              Reporter         // nil: POST /api/reports (F-27) is not served
	ReportKeys           ReportKeys       // nil: GET and POST /api/report-delete (F-37) are not served
	Storage              StorageGate      // nil: no storage gate and GET /api/status is not served (F-10)
	Logger               *slog.Logger     // nil: no log lines
	MaxConcurrentUploads int              // validated by cmd; NewHandler panics on < 1 (a wiring bug)
	ParseTimeout         time.Duration    // the parser's effective time limit; write deadline = 120 s + 6 × it; NewHandler panics on <= 0 when Uploads is set (a wiring bug)
	Now                  func() time.Time // limiter clock; nil: time.Now
	TrustedProxy         netip.Addr       // the one non-loopback peer whose X-Forwarded-For is believed (Caddy, F-31 DP-1a); zero: loopback only
	UploadsPerHourIPv4   int              // per address; validated by cmd; 0: 60
	UploadsPerHourIPv6   int              // per /64; validated by cmd; 0: 60
}

// NewHandler returns the handler for all /api/* routes.
func NewHandler(cfg Config) http.Handler {
	if cfg.MaxConcurrentUploads < 1 {
		panic("httpapi: Config.MaxConcurrentUploads must be at least 1")
	}
	if cfg.Uploads != nil && cfg.ParseTimeout <= 0 {
		panic("httpapi: Config.ParseTimeout must be positive")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}
	mux := http.NewServeMux()
	mux.Handle("GET /api/health", healthHandler(cfg.Version))
	if cfg.Uploads != nil {
		mux.Handle("POST /api/workouts", newUploadHandler(cfg))
	}
	if cfg.Owner != nil {
		registerOwner(mux, cfg)
	}
	if cfg.Reports != nil {
		mux.Handle("POST /api/reports", newReportHandler(cfg))
	}
	if cfg.ReportKeys != nil {
		h := newReportDeleteHandler(cfg)
		mux.Handle("GET /api/report-delete", h)
		mux.Handle("POST /api/report-delete", h)
	}
	if cfg.Storage != nil {
		mux.Handle("GET /api/status", statusHandler(cfg.Storage))
	}
	return mux
}

// healthHandler answers GET and HEAD (ServeMux matches HEAD for GET
// patterns; net/http drops the body for HEAD). It has no side effects.
func healthHandler(version string) http.Handler {
	// The body never changes for the life of the process, so encode it once.
	body, err := json.Marshal(healthResponse{Status: "ok", Version: version})
	if err != nil {
		// Marshalling two strings cannot fail.
		panic(err)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		h := w.Header()
		h.Set("Content-Type", "application/json")
		h.Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})
}
