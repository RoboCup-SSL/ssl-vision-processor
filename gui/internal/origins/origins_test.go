package origins

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func request(method, origin string) *http.Request {
	r := httptest.NewRequest(method, "http://gui.local:8085/api/config/reload", nil)
	if origin != "" {
		r.Header.Set("Origin", origin)
	}

	return r
}

func TestNewRejectsMalformedOrigins(t *testing.T) {
	for _, bad := range []string{"localhost:5173", "http://", "http://a/path", "http://a/?q=1"} {
		if _, err := New(bad); err == nil {
			t.Errorf("New(%q): want an error", bad)
		}
	}
}

func TestAllowed(t *testing.T) {
	p, err := New("http://localhost:5173")
	if err != nil {
		t.Fatal(err)
	}

	for origin, want := range map[string]bool{
		"":                      true,
		"http://gui.local:8085": true,
		"http://localhost:5173": true,
		"HTTP://LOCALHOST:5173": true,
		"http://evil.example":   false,
		"http://localhost:5174": false,
	} {
		if got := p.Allowed(request(http.MethodGet, origin)); got != want {
			t.Errorf("Allowed(Origin %q) = %v, want %v", origin, got, want)
		}
	}
}

func TestHandler(t *testing.T) {
	p, err := New("http://localhost:5173")
	if err != nil {
		t.Fatal(err)
	}

	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	deny := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) })
	h := p.Handler(ok, deny)

	for _, tc := range []struct {
		name, origin string
		want         int
		cors         bool
	}{
		{"same origin", "http://gui.local:8085", http.StatusNoContent, false},
		{"trusted", "http://localhost:5173", http.StatusNoContent, true},
		{"untrusted", "http://evil.example", http.StatusForbidden, false},
		{"no browser", "", http.StatusNoContent, false},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, request(http.MethodPost, tc.origin))

		if rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d", tc.name, rec.Code, tc.want)
		}

		if got := rec.Header().Get("Access-Control-Allow-Origin") != ""; got != tc.cors {
			t.Errorf("%s: CORS headers = %v, want %v", tc.name, got, tc.cors)
		}
	}

	preflight := request(http.MethodOptions, "http://localhost:5173")
	preflight.Header.Set("Access-Control-Request-Method", "PATCH")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, preflight)

	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Errorf("preflight = %d %v, want 204 with allowed headers", rec.Code, rec.Header())
	}
}
