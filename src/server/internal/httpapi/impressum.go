package httpapi

import (
	"hash/maphash"
	"math"
	"net/http"
	"net/netip"
	"strconv"
	"time"
)

// GET /api/impressum (architect/F-38-impressum-page.md IM-4 to IM-6): the
// Impressum as escaped HTML (<h2>, <p>, <ul>), for the /impressum page's
// script only. Contract with that page:
//
//	request: GET, header Wx-Impressum: 1 (browsers add Sec-Fetch-Site: same-origin)
//	200 text/html; charset=utf-8, no-store: the fragment
//	404 the unknown-route answer: header missing or not exactly "1", or Sec-Fetch-Site present and not same-origin
//	429 rate_limited with Retry-After: more than 30 per hour per address (IPv4) or /64 (IPv6)
//
// No log line for any request or refusal; nothing stored (I-7).

const (
	impressumPerHour = 30
	maxImpressumKeys = 1_000
	msgImpressumRate = "Too many requests. Please try again later."
)

type impressumHandler struct {
	body    []byte
	lim     *limiter
	trusted netip.Addr
	seed    maphash.Seed
}

func newImpressumHandler(cfg Config) *impressumHandler {
	return &impressumHandler{body: cfg.Impressum, lim: newLimiter(time.Hour, maxImpressumKeys, cfg.Now), trusted: cfg.TrustedProxy, seed: maphash.MakeSeed()}
}

func (h *impressumHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// IM-5 first: a refusal is byte for byte the unknown-route answer and costs no limiter entry.
	if v := r.Header.Values("Wx-Impressum"); len(v) != 1 || v[0] != "1" {
		http.NotFound(w, r)
		return
	}
	if v := r.Header.Values("Sec-Fetch-Site"); len(v) > 0 && (len(v) != 1 || v[0] != "same-origin") {
		http.NotFound(w, r)
		return
	}
	if ok, retry, _ := h.lim.allow(limiterKey(h.seed, clientPrefix(r, h.trusted)), impressumPerHour); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retry.Seconds()))))
		writeError(w, http.StatusTooManyRequests, codeRateLimited, msgImpressumRate)
		return
	}
	hd := w.Header()
	hd.Set("Content-Type", "text/html; charset=utf-8")
	hd.Set("Cache-Control", "no-store")
	_, _ = w.Write(h.body)
}
