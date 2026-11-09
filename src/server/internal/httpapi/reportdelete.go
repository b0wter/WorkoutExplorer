package httpapi

import (
	"context"
	"errors"
	"hash/maphash"
	"log/slog"
	"math"
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"workout-explorer/internal/reports"
)

// GET and POST /api/report-delete: the delete link's lookup and confirm
// (architect/F-37-report-notification-links.md DK-6 to DK-9, DK-12). The key
// comes only as Authorization: Bearer; POST reads no body, the header is the
// CSRF barrier (ADR-025 item 7). Every failure is the one invalid answer.
//
// Contract with the confirmation page:
//
//	GET  → 200 {"title","start_date","report_id","reason","filed_at","expires_at","report_count"}
//	       (filed_at, expires_at: Unix seconds; reason: the label)
//	POST → 204, no body
//	both → 404 linkInvalidBody; 429 rate_limited with Retry-After; 500 server_error

// ReportKeys looks up and redeems delete keys; *reports.Service satisfies it.
type ReportKeys interface {
	LookupDeleteKey(ctx context.Context, key string) (reports.KeyInfo, error)
	RedeemDeleteKey(ctx context.Context, key string) error
}

// Limits of /api/report-delete (DK-9): the operator is the only legitimate client.
const (
	reportDeletePerHour = 30 // lookup and confirm together, both address families
	maxReportDeleteKeys = 1_000
	msgReportDeleteRate = "Too many requests. Please try again later."
)

// linkInvalidBody is DK-12, byte for byte.
var linkInvalidBody = []byte(`{"error":"not_found","message":"This delete link is no longer valid: it was already used, has expired, or the workout is gone."}`)

func writeLinkInvalid(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write(linkInvalidBody)
}

type deleteKeyBody struct {
	Title       string `json:"title"`
	StartDate   string `json:"start_date"`
	ReportID    int64  `json:"report_id"`
	Reason      string `json:"reason"`
	FiledAt     int64  `json:"filed_at"`
	ExpiresAt   int64  `json:"expires_at"`
	ReportCount int    `json:"report_count"`
}

type reportDeleteHandler struct {
	keys    ReportKeys
	log     *slog.Logger
	lim     *limiter
	trusted netip.Addr
	seed    maphash.Seed
}

func newReportDeleteHandler(cfg Config) *reportDeleteHandler {
	h := &reportDeleteHandler{keys: cfg.ReportKeys, log: cfg.Logger, lim: newLimiter(time.Hour, maxReportDeleteKeys, cfg.Now), trusted: cfg.TrustedProxy, seed: maphash.MakeSeed()}
	h.lim.onFull = func(keys int) { h.log.Warn("report delete rate table full", "keys", keys) }
	return h
}

func (h *reportDeleteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ok, retry, first := h.lim.allow(limiterKey(h.seed, clientPrefix(r, h.trusted)), reportDeletePerHour)
	if !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retry.Seconds()))))
		if first {
			h.log.Info("report delete refused", "reason", codeRateLimited, "status", http.StatusTooManyRequests)
		}
		writeError(w, http.StatusTooManyRequests, codeRateLimited, msgReportDeleteRate)
		return
	}
	if r.URL.RawQuery != "" || r.URL.ForceQuery {
		writeLinkInvalid(w)
		return
	}
	rc := http.NewResponseController(w)
	now := time.Now()
	_ = rc.SetReadDeadline(now.Add(reportTimeout))
	_ = rc.SetWriteDeadline(now.Add(reportTimeout))

	key := bearer(r)
	var err error
	if r.Method == http.MethodPost {
		if err = h.keys.RedeemDeleteKey(r.Context(), key); err == nil {
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusNoContent)
			return
		}
	} else {
		var k reports.KeyInfo
		if k, err = h.keys.LookupDeleteKey(r.Context(), key); err == nil {
			writeJSON(w, http.StatusOK, deleteKeyBody{Title: k.Title, StartDate: k.StartDate, ReportID: k.ReportID, Reason: k.Reason,
				FiledAt: k.FiledAt.Unix(), ExpiresAt: k.ExpiresAt.Unix(), ReportCount: k.ReportCount})
			return
		}
	}
	switch {
	case errors.Is(err, reports.ErrLinkInvalid):
		writeLinkInvalid(w)
	case r.Method != http.MethodPost && r.Context().Err() != nil: // a client that left provokes no ERROR line (ADR-025 item 8)
		writeLinkInvalid(w)
	default:
		// Store and delete errors carry neither the key nor its hash (bound parameters only).
		h.log.Error("report delete failed", "method", r.Method, "error", err.Error())
		writeRefusal(w, codeServerError)
	}
}
