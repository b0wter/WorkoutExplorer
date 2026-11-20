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
	"workout-explorer/internal/preview"
	"workout-explorer/internal/workout"
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
	View(ctx context.Context, rec lifecycle.Record) lifecycle.OwnerView
	PreviewFrame(ctx context.Context, publicID, token string) (lifecycle.MapFrame, error)
	PutPreviewMap(ctx context.Context, publicID, token, key string, body io.Reader) error
}

// Limits of the owner endpoints (F-12 O-12).
const (
	ownerTimeout = 30 * time.Second
	// A 20 MB file on a slow phone: about 33 kB/s.
	ownerDownloadTimeout = 10 * time.Minute
	// The largest valid body: the title and description (1,100 code points)
	// plus 50 pins of 560 code points each (F-56 PN-2), every code point at
	// most 12 bytes escaped (a surrogate pair), plus keys and instants of
	// at most 200 bytes a pin and 1 KiB of other fields: 357,900 bytes.
	maxPatchBytes = 360 << 10
	codeInvalid   = "invalid_settings"
	// F-36 MC-4 refusals.
	codeStale        = "stale"
	codeInvalidImage = "invalid_image"
	msgMapBusy       = "Another preview image is being processed. Please try again."
	msgMapStale      = "The preview frame has changed."
	msgMapRefused    = "The preview image was refused."
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
	Title       string `json:"title"`
	Description string `json:"description"`
	// HideStartM and HideEndM are F-55 HR-4's radii in metres, 0 = not cut.
	HideStartM   int    `json:"hide_start_m"`
	HideEndM     int    `json:"hide_end_m"`
	Visibility   string `json:"visibility"`
	LifetimeDays int    `json:"lifetime_days"`
	CreatedAt    string `json:"created_at"`
	ExpiresAt    string `json:"expires_at"`
	DataEndS     *int   `json:"data_end_s"`
	ViewURL      string `json:"view_url"`
	// HidingSteps is read-only (ADR-056): null without positions or before
	// regenerate filled a migrated row.
	HidingSteps *workout.HidingSteps `json:"hiding_steps"`
	// SuggestedTitle is read-only (F-45 ST-6): the UP-5 suggestion whatever the title, "" = none.
	SuggestedTitle string `json:"suggested_title"`
	// TrimStartAt and TrimEndAt are F-47 TR-3: RFC 3339 UTC, null = that end is not trimmed.
	TrimStartAt *string `json:"trim_start_at"`
	TrimEndAt   *string `json:"trim_end_at"`
	// Trimmed is read-only (F-47 TR-8): the sport and summary the public
	// metadata would have; null without a trim.
	Trimmed *lifecycle.TrimmedView `json:"trimmed"`
	// F-49 ZN-4: the sharer's zone values; null = the file's values.
	ZonesMaxHeartRate   *int `json:"zones_max_heart_rate"`
	ZonesThresholdPower *int `json:"zones_threshold_power"`
	// Pins is F-56 PN-2: the whole stored set, dimmed pins included, in
	// canonical order; [] without pins. The browser counts the pins a Save
	// removed (trim) from this.
	Pins []pinBody `json:"pins"`
}

// pinBody is one pin (F-56 PN-2); instants are RFC 3339 UTC whole seconds.
type pinBody struct {
	Type    string  `json:"type"`
	Title   string  `json:"title"`
	Note    string  `json:"note"`
	StartAt string  `json:"start_at"`
	EndAt   *string `json:"end_at"`
}

func pinsOf(ps []lifecycle.Pin) []pinBody {
	out := make([]pinBody, len(ps))
	for i, p := range ps {
		out[i] = pinBody{Type: p.Type, Title: p.Title, Note: p.Note, StartAt: p.StartAt.UTC().Format(time.RFC3339)}
		if p.EndAt != nil {
			s := p.EndAt.UTC().Format(time.RFC3339)
			out[i].EndAt = &s
		}
	}
	return out
}

// parsePins reads the pins of a PATCH: an array of objects with exactly the
// five keys of PN-2 and the right types; ok is false otherwise. Texts are
// kept as sent (escaped on output, O-4); the rules are lifecycle's.
func parsePins(v json.RawMessage) (pins []lifecycle.Pin, ok bool) {
	var raw []map[string]json.RawMessage
	if json.Unmarshal(v, &raw) != nil || raw == nil || len(raw) > lifecycle.MaxPins {
		return nil, false
	}
	pins = make([]lifecycle.Pin, 0, len(raw))
	for _, m := range raw {
		if len(m) != 5 {
			return nil, false
		}
		var (
			p        lifecycle.Pin
			start    string
			end      *string
			okFields = true
		)
		get := func(k string, dst any) {
			b, present := m[k]
			// null is only for end_at: for a string it would silently unmarshal as "".
			_, isEnd := dst.(**string)
			okFields = okFields && present && (isEnd || string(bytes.TrimSpace(b)) != "null") && json.Unmarshal(b, dst) == nil
		}
		get("type", &p.Type)
		get("title", &p.Title)
		get("note", &p.Note)
		get("start_at", &start)
		get("end_at", &end)
		if !okFields {
			return nil, false
		}
		t, err := time.Parse(time.RFC3339, start)
		if err != nil || t.Nanosecond() != 0 {
			return nil, false
		}
		p.StartAt = t.UTC()
		if end != nil {
			e, err := time.Parse(time.RFC3339, *end)
			if err != nil || e.Nanosecond() != 0 {
				return nil, false
			}
			e = e.UTC()
			p.EndAt = &e
		}
		pins = append(pins, p)
	}
	return pins, true
}

func settingsOf(r lifecycle.Record, v lifecycle.OwnerView) settingsBody {
	at := func(t *time.Time) *string {
		if t == nil {
			return nil
		}
		s := t.UTC().Format(time.RFC3339)
		return &s
	}
	return settingsBody{SuggestedTitle: v.Suggested, Trimmed: v.Trimmed,
		TrimStartAt: at(r.TrimStartAt), TrimEndAt: at(r.TrimEndAt),
		ZonesMaxHeartRate: r.ZonesMaxHeartRate, ZonesThresholdPower: r.ZonesThresholdPower,
		Title: r.Title, Description: r.Description, HideStartM: r.HideStartM, HideEndM: r.HideEndM,
		Visibility: string(r.Visibility), LifetimeDays: r.LifetimeDays(),
		CreatedAt: r.CreatedAt.UTC().Format(time.RFC3339), ExpiresAt: r.ExpiresAt.UTC().Format(time.RFC3339),
		DataEndS: r.DataEndS, ViewURL: lifecycle.ViewPath(r.PublicID),
		HidingSteps: v.Steps, Pins: pinsOf(r.Pins),
	}
}

type ownerHandler struct {
	o Owner
	// build is the write deadline of the requests that run a public build
	// (PATCH, publish; F-15 P-15) and of the owner read: a wait for the lock
	// behind one other build, then the parse time limit, one compression and
	// the writes, three times (a trim PATCH parses to resolve, build and view).
	build time.Duration
}

// buildDeadline is F-15 P-15's write deadline for a parse time limit T.
func buildDeadline(parseTimeout time.Duration) time.Duration {
	return 3 * (ownerTimeout + parseTimeout)
}

func registerOwner(mux *http.ServeMux, cfg Config) {
	h := &ownerHandler{o: cfg.Owner, build: buildDeadline(cfg.ParseTimeout)}
	mux.HandleFunc("GET /api/workouts/{public_id}", h.get)
	mux.HandleFunc("PATCH /api/workouts/{public_id}", h.patch)
	mux.HandleFunc("GET /api/workouts/{public_id}/draft.bin", h.file(false))
	mux.HandleFunc("GET /api/workouts/{public_id}/original.fit", h.file(true))
	mux.HandleFunc("DELETE /api/workouts/{public_id}", h.delete)
	mux.HandleFunc("POST /api/workouts/{public_id}/publish", h.publish)
	mux.HandleFunc("GET /api/workouts/{public_id}/preview-map", h.previewFrame)
	mux.HandleFunc("PUT /api/workouts/{public_id}/preview-map/{key}", h.putPreviewMap)
}

// frameBody is GET …/preview-map (F-36 MC-4): {"frame":null}, or the frame
// with its key and whether a stored basemap matches it.
type frameBody struct {
	Frame   *preview.Frame `json:"frame"`
	Key     string         `json:"key,omitempty"`
	Current *bool          `json:"current,omitempty"`
}

func (h *ownerHandler) previewFrame(w http.ResponseWriter, r *http.Request) {
	id, token, ok := h.begin(w, r, h.build)
	if !ok {
		return
	}
	mf, err := h.o.PreviewFrame(r.Context(), id, token)
	if errors.Is(err, lifecycle.ErrMapBusy) {
		writeError(w, http.StatusServiceUnavailable, codeBusy, msgMapBusy)
		return
	}
	if err != nil {
		fail(w, err)
		return
	}
	var b frameBody
	if mf.OK {
		b = frameBody{Frame: &mf.Frame, Key: mf.Frame.Key, Current: &mf.Current}
	}
	writeJSON(w, http.StatusOK, b)
}

// putPreviewMap is PUT …/preview-map/{key}, body = PNG bytes (F-36 MC-4).
// Refusals are fixed JSON errors the browser ignores.
func (h *ownerHandler) putPreviewMap(w http.ResponseWriter, r *http.Request) {
	id, token, ok := h.begin(w, r, h.build)
	if !ok {
		return
	}
	// One byte over the cap tells lifecycle the body was too large; then the transfer stops.
	body := http.MaxBytesReader(w, r.Body, preview.MaxMapBytes+1)
	err := h.o.PutPreviewMap(r.Context(), id, token, r.PathValue("key"), body)
	var me *lifecycle.MapRefusedError
	switch {
	case err == nil:
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, lifecycle.ErrMapBusy):
		writeError(w, http.StatusServiceUnavailable, codeBusy, msgMapBusy)
	case errors.Is(err, lifecycle.ErrMapStale):
		writeError(w, http.StatusConflict, codeStale, msgMapStale)
	case errors.As(err, &me) && me.Reason == "size":
		writeError(w, http.StatusRequestEntityTooLarge, codeTooLarge, msgMapRefused)
	case errors.As(err, &me):
		writeError(w, http.StatusBadRequest, codeInvalidImage, msgMapRefused)
	default:
		fail(w, err)
	}
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
	writeJSON(w, http.StatusOK, settingsOf(rec, h.o.View(r.Context(), rec)))
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
	id, token, ok := h.begin(w, r, h.build) // a trimmed workout's view parses (F-47 TR-10)
	if !ok {
		return
	}
	rec, err := h.o.Get(r.Context(), id, token)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settingsOf(rec, h.o.View(r.Context(), rec)))
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
	msgBadTrim = "trim_start_at and trim_end_at must be a time in RFC 3339 form with whole seconds, or null."
	msgBadZone = "zones_max_heart_rate and zones_threshold_power must be a whole number, or null."
	msgBadType = "The title and description must be text and lifetime_days a whole number."
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
	writeJSON(w, http.StatusOK, settingsOf(rec, h.o.View(r.Context(), rec)))
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
		case "hide_start_m", "hide_end_m":
			// The range is lifecycle's (F-55 HR-4); a wrong type gets the same sentence.
			o := &p.HideStartM
			if k == "hide_end_m" {
				o = &p.HideEndM
			}
			*o = new(int)
			if bytes.Equal(bytes.TrimSpace(v), []byte("null")) || json.Unmarshal(v, *o) != nil {
				return p, lifecycle.MsgBadHiding
			}
			continue
		case "lifetime_days":
			p.LifetimeDays = new(int)
			dst = p.LifetimeDays
		case "trim_start_at", "trim_end_at":
			// The one exception to "no null" (F-47 TR-3).
			o := &p.TrimStart
			if k == "trim_end_at" {
				o = &p.TrimEnd
			}
			o.Set = true
			if bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
				continue
			}
			var str string
			if json.Unmarshal(v, &str) != nil {
				return p, msgBadTrim
			}
			t, err := time.Parse(time.RFC3339, str)
			if err != nil || t.Nanosecond() != 0 {
				return p, msgBadTrim
			}
			t = t.UTC()
			o.At = &t
			continue
		case "zones_max_heart_rate", "zones_threshold_power":
			// null allowed as for the trim (F-49 ZN-4); the range is lifecycle's.
			o := &p.ZonesMaxHeartRate
			if k == "zones_threshold_power" {
				o = &p.ZonesThresholdPower
			}
			o.Set = true
			if bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
				continue
			}
			o.V = new(int)
			if json.Unmarshal(v, o.V) != nil {
				return p, msgBadZone
			}
			continue
		case "pins":
			// The whole set; null is refused like any other wrong type.
			ps, ok := parsePins(v)
			if !ok {
				return p, lifecycle.MsgBadPins
			}
			p.Pins = lifecycle.OptPins{Set: true, V: ps}
			continue
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
