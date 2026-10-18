// Package httpjson is the host's JSON response conventions: every API reply,
// error or not, is JSON, and every error has the same envelope, so a client
// can branch on a stable code instead of parsing text.
package httpjson

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"slices"
)

// Error codes. Clients branch on these; Message is for people.
const (
	CodeBadRequest        = "bad_request" // malformed request: body, path value, query
	CodeInvalid           = "invalid"     // well-formed, but the content was rejected; see Issues
	CodeNotFound          = "not_found"
	CodeStale             = "stale"               // edited an older revision; refetch and retry
	CodeDiskChanged       = "disk_changed"        // the file changed on disk since it was loaded
	CodeNoLiveCalibration = "no_live_calibration" // nothing received to lock yet
	CodePrecondition      = "precondition"        // a required header (If-Match) is missing
	CodeCrossOrigin       = "cross_origin"        // a browser request on behalf of another site
	CodeUnknownHost       = "unknown_host"        // the request names this host by a name it doesn't answer to
	CodeUnsupportedMedia  = "unsupported_media"   // wrong Content-Type
	CodeInternal          = "internal"            // the host's fault, logged
)

// statuses is each code's HTTP status.
var statuses = map[string]int{
	CodeBadRequest:        http.StatusBadRequest,
	CodeInvalid:           http.StatusBadRequest,
	CodeNotFound:          http.StatusNotFound,
	CodeStale:             http.StatusConflict,
	CodeDiskChanged:       http.StatusConflict,
	CodeNoLiveCalibration: http.StatusConflict,
	CodePrecondition:      http.StatusPreconditionRequired,
	CodeCrossOrigin:       http.StatusForbidden,
	CodeUnknownHost:       http.StatusMisdirectedRequest,
	CodeUnsupportedMedia:  http.StatusUnsupportedMediaType,
	CodeInternal:          http.StatusInternalServerError,
}

// StatusOf is the HTTP status a code is answered with; 500 for one it doesn't
// know.
func StatusOf(code string) int {
	if s, ok := statuses[code]; ok {
		return s
	}

	return http.StatusInternalServerError
}

// Problem is an error a handler returns to answer with a specific code, e.g.
// Errorf(CodeInvalid, "path: required"). Its status is StatusOf(Code).
type Problem struct {
	Code    string
	Message string
	Issues  []string
}

func (p *Problem) Error() string { return p.Message }

// Errorf is a Problem with code and a formatted message.
func Errorf(code, format string, args ...any) *Problem {
	return &Problem{Code: code, Message: fmt.Sprintf(format, args...)}
}

// ErrorBody is the body of every non-2xx API response.
type ErrorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	// Issues lists each problem separately for CodeInvalid.
	Issues []string `json:"issues,omitempty"`
}

// Write sends v as JSON with status.
func Write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("writing response", "err", err)
	}
}

// Error sends an ErrorBody with status.
func Error(w http.ResponseWriter, status int, code, message string, issues ...string) {
	Write(w, status, ErrorBody{Error: code, Message: message, Issues: issues})
}

// NotFound is an http.Handler answering CodeNotFound, for unrouted API paths.
func NotFound() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Error(w, http.StatusNotFound, CodeNotFound, "no such endpoint: "+r.Method+" "+r.URL.Path)
	})
}

// JSONTypes are the media types Decode accepts by default.
var JSONTypes = []string{"application/json"}

// Decode reads r's JSON body into v, answering the request itself and
// returning false on failure. The Content-Type must be one of types
// (JSONTypes if none), and unknown fields are rejected, so a misspelled key
// fails loudly rather than being dropped and wiping the value it named.
func Decode(w http.ResponseWriter, r *http.Request, v any, types ...string) bool {
	if len(types) == 0 {
		types = JSONTypes
	}

	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if !slices.Contains(types, mediaType) {
		Error(w, http.StatusUnsupportedMediaType, CodeUnsupportedMedia, fmt.Sprintf("Content-Type: want one of %v", types))

		return false
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(v); err != nil {
		Error(w, http.StatusBadRequest, CodeBadRequest, "malformed request body: "+err.Error())

		return false
	}

	if decoder.More() {
		Error(w, http.StatusBadRequest, CodeBadRequest, "malformed request body: more than one JSON value")

		return false
	}

	return true
}
