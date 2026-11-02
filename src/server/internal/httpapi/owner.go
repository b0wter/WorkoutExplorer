package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"workout-explorer/internal/lifecycle"
)

// The owner endpoints of F-12 (architect/F-12-owner-api.md, ADR-025). The
// handlers extract the header and map results; the token check is in lifecycle.

// Owner is the owner operations; *lifecycle.Service satisfies it.
type Owner interface {
	Get(ctx context.Context, publicID, token string) (lifecycle.Record, error)
	Update(ctx context.Context, publicID, token string, p lifecycle.SettingsPatch) (lifecycle.Record, error)
	OpenDraft(ctx context.Context, publicID, token string) (lifecycle.Download, error)
	OpenOriginal(ctx context.Context, publicID, token string) (lifecycle.Download, error)
	DeleteByOwner(ctx context.Context, publicID, token string) error
	Publish(ctx context.Context, publicID, token string) (lifecycle.Record, error)
}

// Limits of the owner endpoints (F-12 O-12).
const (
	ownerTimeout = 30 * time.Second
	// A 20 MB file on a slow phone: about 33 kB/s.
	ownerDownloadTimeout = 10 * time.Minute
	// The largest valid body is 1,100 code points, each at most 12 bytes
	// escaped (a surrogate pair), plus the other fields: far below 32 KiB.
	maxPatchBytes = 32 << 10
	codeInvalid   = "invalid_settings"
)

// notFoundBody is ADR-025 item 5, byte for byte.
var notFoundBody = []byte(`{"error":"not_found","message":"This workout has expired or is not available."}`)

func writeNotFound(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write(notFoundBody)
}

// settingsBody is the settings object (F-12 design, Settings object).
type settingsBody struct {
	Title         string `json:"title"`
	Description   string `json:"description"`
	HideEnds      bool   `json:"hide_ends"`
	AllowDownload bool   `json:"allow_download"`
	Visibility    string `json:"visibility"`
	LifetimeDays  int    `json:"lifetime_days"`
	CreatedAt     string `json:"created_at"`
	ExpiresAt     string `json:"expires_at"`
	DataEndS      *int   `json:"data_end_s"`
	ViewURL       string `json:"view_url"`
	// HideEndsRanges is read-only (ADR-030): never nil, so it encodes as [].
	HideEndsRanges [][2]int `json:"hide_ends_ranges"`
}

func settingsOf(r lifecycle.Record) settingsBody {
	return settingsBody{
		Title: r.Title, Description: r.Description, HideEnds: r.HideEnds, AllowDownload: r.AllowDownload,
		Visibility: string(r.Visibility), LifetimeDays: r.LifetimeDays(),
		CreatedAt: r.CreatedAt.UTC().Format(time.RFC3339), ExpiresAt: r.ExpiresAt.UTC().Format(time.RFC3339),
		DataEndS: r.DataEndS, ViewURL: lifecycle.ViewPath(r.PublicID),
		HideEndsRanges: append([][2]int{}, r.HiddenRanges...),
	}
}

type ownerHandler struct {
	o Owner
	// build is the write deadline of the requests that run a public build
	// (PATCH, publish; F-15 P-15): a wait for the lock behind one other
	// build, then the parse time limit, one compression and the writes, twice.
	build time.Duration
}

// buildDeadline is F-15 P-15's write deadline for a parse time limit T.
func buildDeadline(parseTimeout time.Duration) time.Duration {
	return 2 * (ownerTimeout + parseTimeout)
}

func registerOwner(mux *http.ServeMux, cfg Config) {
	h := &ownerHandler{o: cfg.Owner, build: buildDeadline(cfg.ParseTimeout)}
	mux.HandleFunc("GET /api/workouts/{public_id}", h.get)
	mux.HandleFunc("PATCH /api/workouts/{public_id}", h.patch)
	mux.HandleFunc("GET /api/workouts/{public_id}/draft.bin", h.file(false))
	mux.HandleFunc("GET /api/workouts/{public_id}/original.fit", h.file(true))
	mux.HandleFunc("DELETE /api/workouts/{public_id}", h.delete)
	mux.HandleFunc("POST /api/workouts/{public_id}/publish", h.publish)
}

// publish is F-15 P-1: no request body is read.
func (h *ownerHandler) publish(w http.ResponseWriter, r *http.Request) {
	id, token, ok := h.begin(w, r, h.build)
	if !ok {
		return
	}
	rec, err := h.o.Publish(r.Context(), id, token)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settingsOf(rec))
}

// begin applies ADR-025 items 2 and 3 and the deadlines of O-12. ok is false
// if the request was already answered.
func (h *ownerHandler) begin(w http.ResponseWriter, r *http.Request, write time.Duration) (id, token string, ok bool) {
	if r.URL.RawQuery != "" || r.URL.ForceQuery {
		writeNotFound(w)
		return "", "", false
	}
	rc := http.NewResponseController(w)
	now := time.Now()
	_ = rc.SetReadDeadline(now.Add(ownerTimeout))
	_ = rc.SetWriteDeadline(now.Add(write))
	return r.PathValue("public_id"), bearer(r), true
}

// bearer is the token of a single Authorization header with the scheme
// Bearer; anything else is "" (absent, ADR-025 item 2). The value is checked
// by lifecycle.
func bearer(r *http.Request) string {
	v := r.Header.Values("Authorization")
	if len(v) != 1 {
		return ""
	}
	scheme, token, found := strings.Cut(v[0], " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return token
}

// fail maps a lifecycle error. Server errors are logged by lifecycle.
func fail(w http.ResponseWriter, err error) {
	var ie *lifecycle.InvalidSettingsError
	switch {
	case errors.Is(err, lifecycle.ErrNotFound):
		writeNotFound(w)
	case errors.As(err, &ie):
		writeError(w, http.StatusBadRequest, codeInvalid, ie.Message)
	default:
		writeRefusal(w, codeServerError)
	}
}

func (h *ownerHandler) get(w http.ResponseWriter, r *http.Request) {
	id, token, ok := h.begin(w, r, ownerTimeout)
	if !ok {
		return
	}
	rec, err := h.o.Get(r.Context(), id, token)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settingsOf(rec))
}

func (h *ownerHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, token, ok := h.begin(w, r, ownerTimeout)
	if !ok {
		return
	}
	if err := h.o.DeleteByOwner(r.Context(), id, token); err != nil {
		fail(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func (h *ownerHandler) file(original bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, token, ok := h.begin(w, r, ownerDownloadTimeout)
		if !ok {
			return
		}
		open := h.o.OpenDraft
		if original {
			open = h.o.OpenOriginal
		}
		d, err := open(r.Context(), id, token)
		if err != nil {
			fail(w, err)
			return
		}
		defer d.Body.Close()
		hd := w.Header()
		hd.Set("Content-Type", "application/octet-stream")
		hd.Set("Content-Length", strconv.FormatInt(d.Size, 10))
		hd.Set("Cache-Control", "no-store")
		hd.Set("X-Content-Type-Options", "nosniff")
		if d.Name != "" {
			hd.Set("Content-Disposition", `attachment; filename="`+d.Name+`"`)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, d.Body) // a broken connection ends the copy; nothing to report
	}
}

const (
	msgBadBody = "The request must be one JSON object with only the settings that can be changed."
	msgBadType = "The title and description must be text, hide_ends and allow_download true or false, and lifetime_days a whole number."
)

func (h *ownerHandler) patch(w http.ResponseWriter, r *http.Request) {
	id, token, ok := h.begin(w, r, h.build)
	if !ok {
		return
	}
	// The token check comes first: a stranger learns nothing from the body rules.
	if _, err := h.o.Get(r.Context(), id, token); err != nil {
		fail(w, err)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxPatchBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, codeInvalid, msgBadBody)
		return
	}
	p, msg := parsePatch(body)
	if msg != "" {
		writeError(w, http.StatusBadRequest, codeInvalid, msg)
		return
	}
	rec, err := h.o.Update(r.Context(), id, token, p)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settingsOf(rec))
}

// parsePatch reads the body of a PATCH: one JSON object whose keys are
// settable fields, no null, right types (F-12 design). msg is the refusal's
// sentence, empty if the body is valid.
func parsePatch(body []byte) (p lifecycle.SettingsPatch, msg string) {
	dec := json.NewDecoder(bytes.NewReader(body))
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil || raw == nil {
		return p, msgBadBody
	}
	if _, err := dec.Token(); err != io.EOF { // nothing may follow the object
		return p, msgBadBody
	}
	if hasDuplicateKey(body) {
		return p, msgBadBody
	}
	for k, v := range raw {
		var dst any
		switch k {
		case "title":
			p.Title = new(string)
			dst = p.Title
		case "description":
			p.Description = new(string)
			dst = p.Description
		case "hide_ends":
			p.HideEnds = new(bool)
			dst = p.HideEnds
		case "allow_download":
			p.AllowDownload = new(bool)
			dst = p.AllowDownload
		case "lifetime_days":
			p.LifetimeDays = new(int)
			dst = p.LifetimeDays
		default:
			return p, msgBadBody
		}
		if bytes.Equal(bytes.TrimSpace(v), []byte("null")) || json.Unmarshal(v, dst) != nil {
			return p, msgBadType
		}
	}
	return p, ""
}

// hasDuplicateKey reports whether the top-level object repeats a key
// (QA-12-r1-5). The body is already known to be one valid JSON object.
func hasDuplicateKey(body []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(body))
	if _, err := dec.Token(); err != nil { // {
		return false
	}
	seen := map[string]bool{}
	for dec.More() {
		k, err := dec.Token()
		if err != nil {
			return false
		}
		key, _ := k.(string)
		if seen[key] {
			return true
		}
		seen[key] = true
		var skip json.RawMessage
		if dec.Decode(&skip) != nil {
			return false
		}
	}
	return false
}
