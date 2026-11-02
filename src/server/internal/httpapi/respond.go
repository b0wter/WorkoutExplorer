package httpapi

import (
	"encoding/json"
	"net/http"
)

// refusal is one row of the response table of the F-09 design: the HTTP
// status and the sentence for a code. The sentences for refusals that are not
// about the file's content are UP-10's, exactly. Ingest's own messages are
// never copied here (ADR-018).
type refusal struct {
	status  int
	message string
}

const (
	codeBusy        = "busy"
	codeRateLimited = "rate_limited"
	codeBadRequest  = "bad_request"
	codeTooLarge    = "too_large"
	codeServerError = "server_error"
	codeStorageFull = "storage_full"
)

var refusals = map[string]refusal{
	codeBusy:        {http.StatusServiceUnavailable, "Too many uploads right now. Please try again in a minute."},
	codeRateLimited: {http.StatusTooManyRequests, "Too many uploads. Please try again later."},
	codeBadRequest:  {http.StatusBadRequest, "The upload could not be read. Please try again."},
	codeTooLarge:    {http.StatusRequestEntityTooLarge, "This file is larger than 20 MB."},
	codeServerError: {http.StatusInternalServerError, "Something went wrong on the server. Please try again later."},
	codeStorageFull: {http.StatusServiceUnavailable, "Uploads are paused because storage is full. Please try again later."}, // UP-8
}

type errorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type createdBody struct {
	ViewURL string `json:"view_url"`
	EditURL string `json:"edit_url"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	b, err := json.Marshal(v)
	if err != nil { // two-string structs cannot fail
		panic(err)
	}
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

// writeRefusal answers with the table row for code.
func writeRefusal(w http.ResponseWriter, code string) {
	r := refusals[code]
	writeJSON(w, r.status, errorBody{Error: code, Message: r.message})
}

// writeError answers with an explicit status, code and message (ingest's).
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: code, Message: message})
}
