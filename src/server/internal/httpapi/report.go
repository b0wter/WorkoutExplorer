package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"hash/maphash"
	"io"
	"log/slog"
	"math"
	"mime"
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"workout-explorer/internal/reports"
)

// POST /api/reports (architect/F-27-reports-api-and-operator-delete.md R-5,
// R-6). Nothing about the request is stored or logged.

// Reporter files a report; *reports.Service satisfies it.
type Reporter interface {
	File(ctx context.Context, publicID, reason, message string) error
}

// Limits of POST /api/reports.
const (
	reportsPerHour  = 60 // both address families; a constant, not a setting (decision log 1 Nov 2026)
	maxReportKeys   = 10_000
	maxReportBytes  = 16 << 10
	reportTimeout   = 30 * time.Second
	codeInvalidRep  = "invalid_report"
	msgReportType   = "The report must be sent as JSON."
	msgReportBody   = "The report must be one JSON object with a public_id, a reason and an optional message, all as text."
	msgReportsLimit = "Too many reports. Please try again later."
)

type reportHandler struct {
	rep     Reporter
	log     *slog.Logger
	lim     *limiter
	trusted netip.Addr
	seed    maphash.Seed
}

func newReportHandler(cfg Config) *reportHandler {
	h := &reportHandler{rep: cfg.Reports, log: cfg.Logger, lim: newLimiter(time.Hour, maxReportKeys, cfg.Now), trusted: cfg.TrustedProxy, seed: maphash.MakeSeed()}
	h.lim.onFull = func(keys int) { h.log.Warn("report rate table full", "keys", keys) }
	return h
}

func (h *reportHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ok, retry, first := h.lim.allow(limiterKey(h.seed, clientPrefix(r, h.trusted)), reportsPerHour)
	if !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retry.Seconds()))))
		if first {
			h.log.Info("report refused", "reason", codeRateLimited, "status", http.StatusTooManyRequests)
		}
		writeError(w, http.StatusTooManyRequests, codeRateLimited, msgReportsLimit)
		return
	}
	rc := http.NewResponseController(w)
	now := time.Now()
	_ = rc.SetReadDeadline(now.Add(reportTimeout))
	_ = rc.SetWriteDeadline(now.Add(reportTimeout))

	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		writeError(w, http.StatusBadRequest, codeInvalidRep, msgReportType)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxReportBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidRep, msgReportBody)
		return
	}
	f, ok := parseReport(body)
	if !ok {
		writeError(w, http.StatusBadRequest, codeInvalidRep, msgReportBody)
		return
	}
	var ie *reports.InvalidError
	switch err := h.rep.File(r.Context(), f["public_id"], f["reason"], f["message"]); {
	case err == nil:
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
	case errors.As(err, &ie):
		writeError(w, http.StatusBadRequest, codeInvalidRep, ie.Message)
	case errors.Is(err, reports.ErrNotFound):
		writeNotFound(w)
	default:
		if r.Context().Err() == nil { // a client that left provokes no ERROR line (ADR-025 item 8)
			h.log.Error("report failed", "error", err.Error())
		}
		writeRefusal(w, codeServerError)
	}
}

// parseReport reads one JSON object with the string keys public_id and
// reason and the optional string message: no other key, no duplicate, no
// null, nothing after the object (the F-12 PATCH rules).
func parseReport(body []byte) (map[string]string, bool) {
	dec := json.NewDecoder(bytes.NewReader(body))
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil || raw == nil {
		return nil, false
	}
	if _, err := dec.Token(); err != io.EOF || hasDuplicateKey(body) {
		return nil, false
	}
	out := map[string]string{}
	for k, v := range raw {
		if k != "public_id" && k != "reason" && k != "message" {
			return nil, false
		}
		var s string
		if bytes.Equal(bytes.TrimSpace(v), []byte("null")) || json.Unmarshal(v, &s) != nil {
			return nil, false
		}
		out[k] = s
	}
	_, id := out["public_id"]
	_, reason := out["reason"]
	return out, id && reason
}
