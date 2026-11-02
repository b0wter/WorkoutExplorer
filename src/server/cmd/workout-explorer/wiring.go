package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"time"

	"workout-explorer/internal/httpapi"
	"workout-explorer/internal/ingest"
	"workout-explorer/internal/lifecycle"
	"workout-explorer/internal/privatedir"
	"workout-explorer/internal/publish"
	"workout-explorer/internal/store"
)

// settings are the values of a server run that do not come from flags alone.
type settings struct {
	maxConcurrentUploads int
	parseTimeout         time.Duration // 0: ingest.DefaultLimits.Timeout
	storageLimit         int64         // bytes; 0: the setting's default (F-10)
	webDir               string        // -web: the deployed web build; <webDir>/index.html is the page template (F-15 P-10)
	baseURL              string        // origin of absolute links (F-23 PC-8)
	trustedProxy         netip.Addr    // Caddy's address (F-31 DP-1a); zero: loopback only
	uploadsIPv4          int           // uploads per hour per IPv4 address; 0: 60 (tests)
	uploadsIPv6          int           // per IPv6 /64; 0: 60 (tests)
	// onParser is called with the parser newApp built; nil in production.
	onParser func(*ingest.Parser)
	// onService is called with the service and the storage total newApp
	// built (F-10 tests reach Delete through it); nil in production.
	onService func(*lifecycle.Service, *lifecycle.Storage)
	// wrapUploader lets a test stand between the handler and the service
	// (for example to hold uploads in flight); nil in production.
	wrapUploader func(httpapi.Uploader) httpapi.Uploader
	// expiryEvery is the period of the expiry pass; 0: expiryPeriod. Tests only.
	expiryEvery time.Duration
}

// expiryPeriod leaves room in SH-10's 60 s for the lock waits of F-21 X-6.
const expiryPeriod = 30 * time.Second

// newApp is the one wiring, used by serve and by the end-to-end tests: it
// loads the page template, creates the data directory, opens the database,
// the private files and the public area, builds the parser and the service,
// runs the startup recovery and the first expiry pass, starts the expiry
// timer (F-21 X-5) and returns the handler and the func that stops the timer
// and closes the database.
func newApp(ctx context.Context, dataDir string, s settings, logger *slog.Logger) (http.Handler, func() error, error) {
	// The template is read once, before anything on disk changes (F-15 P-10).
	tmpl, err := publish.LoadTemplate(s.webDir)
	if err != nil {
		return nil, nil, err
	}
	if s.baseURL == "" {
		return nil, nil, errBaseURL
	}
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return nil, nil, fmt.Errorf("data directory: %w", err)
	}
	db, err := store.Open(ctx, filepath.Join(dataDir, "workouts.db"))
	if err != nil {
		return nil, nil, err
	}
	fail := func(err error) (http.Handler, func() error, error) {
		db.Close()
		return nil, nil, err
	}
	files, err := privatedir.Open(dataDir)
	if err != nil {
		return fail(err)
	}
	parser := newParser(s.parseTimeout, logger)
	if s.onParser != nil {
		s.onParser(parser)
	}
	limit := s.storageLimit
	if limit == 0 {
		limit = defaultStorageLimitMB * bytesPerMB
	}
	// The total starts at 0 and gets the measured usage after recovery, so
	// removed orphans and staging never count (ADR-022, F-10 design).
	storage := lifecycle.NewStorage(limit, 0)
	public, err := publish.Open(dataDir, publish.Deps{Template: tmpl, Originals: files, Parser: parser, Storage: storage, Logger: logger, BaseURL: s.baseURL})
	if err != nil {
		return fail(err)
	}
	svc := lifecycle.New(lifecycle.Deps{
		Parser:  parser,
		Repo:    db,
		Files:   files,
		Storage: storage,
		Hook:    public,
		Public:  public,
		Logger:  logger,
	})
	if err := svc.Recover(ctx); err != nil {
		return fail(err)
	}
	// Before the total is measured and the listener opens (F-21 AC 6). A
	// failure is logged by Expire and retried by the timer: the owner API
	// answers a due row as not found, and the sweep removes its public folder.
	_ = svc.Expire(ctx)
	used, err := privatedir.Usage(dataDir)
	if err != nil {
		return fail(fmt.Errorf("storage usage: %w", err))
	}
	storage.Add(used)
	logger.Info("storage", "used_bytes", used, "limit_bytes", limit, "uploads_open", storage.UploadsOpen())
	if s.onService != nil {
		s.onService(svc, storage)
	}
	var uploads httpapi.Uploader = svc
	if s.wrapUploader != nil {
		uploads = s.wrapUploader(uploads)
	}
	h := httpapi.NewHandler(httpapi.Config{
		Version:              version,
		Uploads:              uploads,
		Owner:                svc,
		Storage:              storage,
		Logger:               logger,
		MaxConcurrentUploads: s.maxConcurrentUploads,
		ParseTimeout:         parser.Limits().Timeout,
		TrustedProxy:         s.trustedProxy,
		UploadsPerHourIPv4:   s.uploadsIPv4,
		UploadsPerHourIPv6:   s.uploadsIPv6,
	})
	every := s.expiryEvery
	if every == 0 {
		every = expiryPeriod
	}
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-t.C:
				_ = svc.Expire(context.WithoutCancel(ctx)) // Expire logs; the next tick retries
			}
		}
	}()
	var once sync.Once
	closeApp := func() error {
		once.Do(func() { close(done) })
		wg.Wait() // a running pass finishes before the database closes
		return db.Close()
	}
	return h, closeApp, nil
}
