package httpapi

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"hash/maphash"
	"io"
	"log/slog"
	"math"
	"mime"
	"mime/multipart"
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"workout-explorer/internal/ingest"
	"workout-explorer/internal/lifecycle"
)

// Limits of POST /api/workouts (ADR-021).
const (
	// MaxUploadBytes is "20 MB" of file content: decimal megabytes (OQ-43).
	MaxUploadBytes = 20_000_000
	// maxBodyBytes adds room for the multipart framing.
	maxBodyBytes     = MaxUploadBytes + 16<<10
	defaultRateLimit = 60 // per hour; Config zero value (tests); cmd always sets both
	uploadRateWindow = time.Hour
	maxRateKeys      = 100_000
)

// Per-request deadlines (ADR-021 item 5). Variables only so a test can
// shorten them.
// The write deadline is the read deadline plus uploadWriteFactor times the
// parser's time limit (ADR-021 rev. 4): 180 s at the default 10 s.
var uploadReadTimeout = 120 * time.Second

const uploadWriteFactor = 6

// Uploader turns the bytes of an upload into a stored draft.
type Uploader interface {
	Upload(ctx context.Context, upload []byte) (lifecycle.Created, error)
}

// uploadHandler is POST /api/workouts (F-09 design, section POST
// /api/workouts). Its steps are numbered as in the design's order of checks.
type uploadHandler struct {
	up      Uploader
	storage StorageGate // nil: no gate
	log     *slog.Logger
	slots   chan struct{}
	lim     *limiter
	trusted netip.Addr
	limit4  int // uploads per hour per IPv4 address
	limit6  int // per IPv6 /64
	seed    maphash.Seed
	maxBody int64

	parseTimeout time.Duration
}

func newUploadHandler(cfg Config) *uploadHandler {
	h := &uploadHandler{
		up:           cfg.Uploads,
		storage:      cfg.Storage,
		parseTimeout: cfg.ParseTimeout,
		log:          cfg.Logger,
		slots:        make(chan struct{}, cfg.MaxConcurrentUploads),
		lim:          newLimiter(uploadRateWindow, maxRateKeys, cfg.Now),
		trusted:      cfg.TrustedProxy,
		limit4:       cmp.Or(cfg.UploadsPerHourIPv4, defaultRateLimit),
		limit6:       cmp.Or(cfg.UploadsPerHourIPv6, defaultRateLimit),
		seed:         maphash.MakeSeed(),
		maxBody:      maxBodyBytes,
	}
	h.lim.onFull = func(keys int) { h.log.Warn("rate table full", "keys", keys) }
	return h
}

func (h *uploadHandler) refuse(w http.ResponseWriter, code string) {
	h.log.Info("upload refused", "reason", code, "status", refusals[code].status)
	writeRefusal(w, code)
}

func (h *uploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 0 (F-10): the storage gate, before the in-flight slot and the rate
	// limit, so a refusal here does not count against the limit.
	if h.storage != nil && !h.storage.UploadsOpen() {
		h.refuse(w, codeStorageFull)
		return
	}

	// 1: an in-flight slot, released when the handler returns.
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		w.Header().Set("Retry-After", "30")
		h.refuse(w, codeBusy)
		return
	}

	// 2: the rate limit; this attempt counts if allowed.
	pfx := clientPrefix(r, h.trusted)
	limit := h.limit6
	if pfx.Addr().Is4() {
		limit = h.limit4
	}
	ok, retry, first := h.lim.allow(limiterKey(h.seed, pfx), limit)
	if !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retry.Seconds()))))
		if first {
			h.log.Info("upload refused", "reason", codeRateLimited, "status", http.StatusTooManyRequests)
		}
		writeRefusal(w, codeRateLimited)
		return
	}

	// 3: deadlines. A recorder that cannot set them is fine; a real server can.
	// A failure is a WARN, not an ERROR: the usual cause is a connection the
	// client has already closed, and the handler goes on (a broken connection
	// fails at step 6). The error carries only the local address.
	rc := http.NewResponseController(w)
	now := time.Now()
	if err := rc.SetReadDeadline(now.Add(uploadReadTimeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		h.log.Warn("upload failed", "stage", "deadline", "error", err.Error())
	}
	if err := rc.SetWriteDeadline(now.Add(uploadReadTimeout + uploadWriteFactor*h.parseTimeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		h.log.Warn("upload failed", "stage", "deadline", "error", err.Error())
	}

	// 4: multipart/form-data with a boundary.
	mt, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "multipart/form-data" || params["boundary"] == "" {
		h.refuse(w, codeBadRequest)
		return
	}

	// 5: refused before a single body byte is read.
	if r.ContentLength > h.maxBody {
		h.refuse(w, codeTooLarge)
		return
	}

	// 6: stream the first part into memory, never through ParseMultipartForm.
	r.Body = http.MaxBytesReader(w, r.Body, h.maxBody)
	mr := multipart.NewReader(r.Body, params["boundary"])
	part, err := mr.NextPart()
	if err != nil {
		h.refuse(w, readFailure(err))
		return
	}
	if part.FormName() != "file" {
		h.refuse(w, codeBadRequest)
		return
	}
	var buf bytes.Buffer
	if r.ContentLength > 0 {
		buf.Grow(int(min(r.ContentLength, MaxUploadBytes)))
	}
	if _, err := io.Copy(&buf, io.LimitReader(part, MaxUploadBytes+1)); err != nil {
		h.refuse(w, readFailure(err))
		return
	}
	if buf.Len() > MaxUploadBytes {
		h.refuse(w, codeTooLarge)
		return
	}

	// 7: exactly one part.
	if _, err := mr.NextPart(); err != io.EOF {
		if err == nil {
			h.refuse(w, codeBadRequest)
		} else {
			h.refuse(w, readFailure(err))
		}
		return
	}

	// 8: store.
	created, err := h.up.Upload(r.Context(), buf.Bytes())
	if err != nil {
		h.fail(w, err)
		return
	}

	// 9: the two links; the token only in the fragment.
	writeJSON(w, http.StatusCreated, createdBody{
		ViewURL: lifecycle.ViewPath(created.PublicID),
		EditURL: lifecycle.EditLink(created.PublicID, created.EditToken),
	})
}

// readFailure maps an error while reading the body: the byte cap is 413,
// everything else (malformed multipart, a body cut off) is 400.
func readFailure(err error) string {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		return codeTooLarge
	}
	return codeBadRequest
}

// fail maps an Upload error (F-09 design, response table).
func (h *uploadHandler) fail(w http.ResponseWriter, err error) {
	var ie *ingest.Error
	switch {
	case errors.As(err, &ie) && ie.Kind == ingest.KindCanceled:
		writeError(w, http.StatusBadRequest, string(ie.Kind), ie.Message())
	case errors.As(err, &ie):
		// Parser rejections are not logged a second time (ADR-018).
		writeError(w, http.StatusUnprocessableEntity, string(ie.Kind), ie.Message())
	case errors.Is(err, lifecycle.ErrStorageFull):
		// The cap was reached while this upload was being processed (F-10).
		h.refuse(w, codeStorageFull)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		// The request ended before the files were written: the client is gone.
		writeRefusal(w, codeBadRequest)
	default:
		writeRefusal(w, codeServerError)
	}
}
