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

	"workout-explorer/internal/about"
	"workout-explorer/internal/assets"
	"workout-explorer/internal/httpapi"
	"workout-explorer/internal/ingest"
	"workout-explorer/internal/lifecycle"
	"workout-explorer/internal/notify"
	"workout-explorer/internal/places"
	"workout-explorer/internal/privatedir"
	"workout-explorer/internal/publish"
	"workout-explorer/internal/reports"
	"workout-explorer/internal/shell"
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
	pushoverToken        string        // both set: report notifications on (ADR-039); secrets, never logged or printed
	pushoverUser         string
	pushoverEndpoint     string      // "": notify.Endpoint; tests point it at a fake
	notifyKind           notify.Kind // ntfy or callback (F-70); "": neither. Pushover keys and a kind never come together
	notifyURL            string      // secret: the whole URL; only its host is logged or shown (NS-8)
	notifyToken          string      // secret: bearer token, optional
	operatorFile         string      // WORKOUT_EXPLORER_OPERATOR_FILE, for the missing-file warning
	operatorHTML         string      // the rendered operator file (F-29 AB-8); "": the placeholder
	iosShortcutURL       string      // the iOS Shortcut link of the About page (F-34) and the upload hint (F-63); "": neither
	impressumHTML        string      // the rendered Impressum file (F-38 IM-1); "": no Impressum, no page, no footer link
	autoUpdate           bool        // WORKOUT_EXPLORER_AUTO_UPDATE=on: the About page's update sentence (F-52 AU-12, PT-9a)
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

// notifyConfig is the notifier's configuration; ok false: notifications are off.
func (s settings) notifyConfig() (c notify.Config, ok bool) {
	switch {
	case s.notifyKind != "":
		return notify.Config{Kind: s.notifyKind, URL: s.notifyURL, Token: s.notifyToken}, true
	case s.pushoverToken != "":
		return notify.Config{Kind: notify.Pushover, URL: s.pushoverEndpoint, Token: s.pushoverToken, User: s.pushoverUser}, true
	}
	return notify.Config{}, false
}

// notifyService is what the About page may name: the kind and, for ntfy, the host (NS-8).
func (s settings) notifyService() about.Notify {
	c, ok := s.notifyConfig()
	switch {
	case !ok:
		return about.Notify{}
	case c.Kind == notify.Ntfy:
		return about.Notify{Kind: string(c.Kind), Host: notify.Host(c.URL)}
	}
	return about.Notify{Kind: string(c.Kind)}
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
	impressum := s.impressumHTML != ""
	tmpl, err := publish.LoadTemplate(s.webDir, impressum)
	if err != nil {
		return nil, nil, err
	}
	aboutPage, err := about.Load(s.webDir, about.Values{Operator: s.operatorHTML, Version: version, Notify: s.notifyService(), Shortcut: s.iosShortcutURL, Impressum: impressum, Updates: s.autoUpdate})
	if err != nil {
		return nil, nil, err
	}
	if s.operatorHTML == "" {
		logger.Warn("operator file missing: the About page shows a placeholder", "setting", envOperatorFile, "set", s.operatorFile != "")
	}
	if s.baseURL == "" {
		return nil, nil, errBaseURL
	}
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return nil, nil, fmt.Errorf("data directory: %w", err)
	}
	// ADR-053: this build's assets, before the shells that name them and before the listener opens.
	if _, err := assets.Copy(dataDir, s.webDir); err != nil {
		return nil, nil, err
	}
	// ADR-043: the page shells Caddy serves, filled for this start, before the listener opens.
	if err := shell.Write(dataDir, s.webDir, impressum, s.iosShortcutURL); err != nil {
		return nil, nil, err
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
	parser := newParser(s.parseTimeout, false, logger)
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
	public, err := publish.Open(dataDir, publish.Deps{Template: tmpl, Originals: files, Maps: files, Parser: parser, Storage: storage, Logger: logger, BaseURL: s.baseURL, Rows: db, Places: places.Default()})
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
		Frames:  public,
		Trimmer: public,
		Places:  places.Default(),
		Logger:  logger,
	})
	if err := svc.Recover(ctx); err != nil {
		return fail(err)
	}
	// Reports (F-27): notifications only when both secrets are set; each sent
	// one carries a delete link issued by reports (F-37 DK-3).
	var notifier *notify.Notifier
	var reps *reports.Service
	ncfg, notifyOn := s.notifyConfig()
	if notifyOn {
		notifier = notify.New(ncfg, func(ctx context.Context, id int64) (string, bool) {
			return reps.IssueDeleteLink(ctx, id)
		}, func(ctx context.Context, id int64) { reps.RevokeDeleteLink(ctx, id) }, logger)
	}
	deps := reports.Deps{Store: db, Deleter: svc, BaseURL: s.baseURL, Logger: logger}
	if notifier != nil {
		deps.Notifier = notifier
	}
	reps = reports.New(deps)
	// F-37 DK-5: with notifications off no delete link may work; F-70 NS-13: nor
	// after a change of service, URL or token. The state file is written after
	// the clearing, so a crash in between clears again.
	commit, err := notifyChanged(dataDir, s)
	if err != nil {
		return fail(err)
	}
	if notifier == nil || commit != nil {
		if err := reps.ClearDeleteKeys(ctx); err != nil {
			return fail(err)
		}
	}
	if commit != nil {
		if err := commit(); err != nil {
			return fail(err)
		}
	}
	notifyCtx, stopNotify := context.WithCancel(context.WithoutCancel(ctx))
	notifyDone := make(chan struct{})
	if notifier != nil {
		go func() {
			defer close(notifyDone)
			notifier.Run(notifyCtx) // pending notifications are dropped at shutdown (ADR-039 item 3)
		}()
	} else {
		close(notifyDone)
	}
	// expire is one pass: reports past R-3's ceiling, then the workouts,
	// whose checkpoint also takes the pruned reports out of the WAL.
	expire := func(ctx context.Context) {
		reps.Prune(ctx)
		_ = svc.Expire(ctx) // Expire logs; the next tick retries
	}
	// Before the total is measured and the listener opens (F-21 AC 6). A
	// failure is logged by Expire and retried by the timer: the owner API
	// answers a due row as not found, and the sweep removes its public folder.
	expire(ctx)
	used, err := privatedir.Usage(dataDir)
	if err != nil {
		stopNotify()
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
		Reports:              reps,
		ReportKeys:           reps,
		Storage:              storage,
		Logger:               logger,
		MaxConcurrentUploads: s.maxConcurrentUploads,
		ParseTimeout:         parser.Limits().Timeout,
		TrustedProxy:         s.trustedProxy,
		UploadsPerHourIPv4:   s.uploadsIPv4,
		UploadsPerHourIPv6:   s.uploadsIPv6,
		Impressum:            []byte(s.impressumHTML),
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
				expire(context.WithoutCancel(ctx))
			}
		}
	}()
	var once sync.Once
	closeApp := func() error {
		once.Do(func() { close(done); stopNotify() })
		wg.Wait() // a running pass finishes before the database closes
		// QA-70-r1-1: an in-flight send ends (cancelled) and its revoke runs (5 s deadline) before the database closes.
		<-notifyDone
		return db.Close()
	}
	mux := http.NewServeMux()
	mux.Handle("GET /about", about.Handler(aboutPage)) // F-29 AB-4; no access log
	mux.Handle("/", h)
	return mux, closeApp, nil
}
