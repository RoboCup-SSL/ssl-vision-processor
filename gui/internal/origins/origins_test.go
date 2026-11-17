package origins

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func request(method, host, origin string) *http.Request {
	r := httptest.NewRequest(method, "http://"+host+"/api/config/reload", nil)
	if origin != "" {
		r.Header.Set("Origin", origin)
	}

	return r
}

func policy(t *testing.T, opts Options) *Policy {
	t.Helper()

	p, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}

	return p
}

func TestNewRejectsMalformedOrigins(t *testing.T) {
	for _, bad := range []string{"localhost:5173", "http://", "http://a/path", "http://a/?q=1"} {
		if _, err := New(Options{TrustedOrigins: []string{bad}}); err == nil {
			t.Errorf("New(%q): want an error", bad)
		}
	}
}

func TestAllowedInTheLab(t *testing.T) {
	p := policy(t, Options{TrustedOrigins: []string{"https://ui.example.org"}})

	for origin, want := range map[string]bool{
		"":                            true, // not a browser
		"http://vision-pc:8085":       true, // the host's own page
		"http://localhost:5173":       true,
		"http://127.0.0.1:5173":       true,
		"http://192.168.1.20:3000":    true,
		"http://10.0.0.7":             true,
		"http://[fe80::1]:3000":       true,
		"http://laptop.local:5173":    true,
		"http://laptop:5173":          true,
		"https://ui.example.org":      true, // listed
		"http://evil.example":         false,
		"http://8.8.8.8":              false,
		"http://192.168.1.20.evil.io": false,
	} {
		if got := p.Allowed(request(http.MethodGet, "vision-pc:8085", origin)); got != want {
			t.Errorf("lab: Allowed(Origin %q) = %v, want %v", origin, got, want)
		}
	}
}

func TestAllowedOnACompetitionField(t *testing.T) {
	p := policy(t, Options{TrustedOrigins: []string{"https://ui.example.org"}})
	p.SetStrict(true)

	for origin, want := range map[string]bool{
		"":                         true,
		"http://vision-pc:8085":    true,
		"https://ui.example.org":   true,
		"http://192.168.1.20:3000": false,
		"http://localhost:5173":    false,
		"http://evil.example":      false,
	} {
		if got := p.Allowed(request(http.MethodGet, "vision-pc:8085", origin)); got != want {
			t.Errorf("strict: Allowed(Origin %q) = %v, want %v", origin, got, want)
		}
	}

	p.SetStrict(false)

	if !p.Allowed(request(http.MethodGet, "vision-pc:8085", "http://192.168.1.20:3000")) {
		t.Error("back in the lab: a LAN origin should be allowed again")
	}
}

func TestTrustAll(t *testing.T) {
	p := policy(t, Options{TrustedOrigins: []string{"*"}})
	p.SetStrict(true)

	if !p.Allowed(request(http.MethodGet, "vision-pc:8085", "http://evil.example")) {
		t.Error(`"*" should trust every origin`)
	}
}

func TestHostAllowed(t *testing.T) {
	p := policy(t, Options{AllowedHosts: []string{"vision.example.edu"}})
	p.AddOwnHosts("vision-pc.cs.example.edu.")

	for host, want := range map[string]bool{
		"192.168.1.50:8085":             true,
		"203.0.113.9:8085":              true, // any IP literal
		"[::1]:8085":                    true,
		"localhost:8085":                true,
		"vision-pc:8085":                true,
		"vision-pc.local:8085":          true,
		"vision-pc.cs.example.edu:8085": true,  // its own reverse-DNS name
		"VISION.example.edu":            true,  // listed alias
		"evil.example:8085":             false, // a rebinding attacker's name
		"vision-pc.evil.example":        false,
	} {
		if got := p.HostAllowed(request(http.MethodGet, host, "")); got != want {
			t.Errorf("HostAllowed(%q) = %v, want %v", host, got, want)
		}
	}

	if !policy(t, Options{AllowedHosts: []string{"*"}}).HostAllowed(request(http.MethodGet, "evil.example", "")) {
		t.Error(`"*" should turn the host check off`)
	}
}

func TestHandler(t *testing.T) {
	p := policy(t, Options{})

	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	deny := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) })
	badHost := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusMisdirectedRequest) })
	h := p.Handler(ok, deny, badHost)

	for _, tc := range []struct {
		name, host, origin, site string
		want                     int
		cors                     bool
	}{
		{"same origin", "vision-pc:8085", "http://vision-pc:8085", "", http.StatusNoContent, false},
		{"same origin by fetch metadata", "vision-pc:8085", "", "same-origin", http.StatusNoContent, false},
		{"lab UI on the LAN", "vision-pc:8085", "http://192.168.1.20:3000", "cross-site", http.StatusNoContent, true},
		{"public site", "vision-pc:8085", "http://evil.example", "cross-site", http.StatusForbidden, false},
		{"cross-site without Origin", "vision-pc:8085", "", "cross-site", http.StatusForbidden, false},
		{"no browser", "vision-pc:8085", "", "", http.StatusNoContent, false},
		{"DNS rebinding", "evil.example:8085", "http://evil.example:8085", "same-origin", http.StatusMisdirectedRequest, false},
	} {
		req := request(http.MethodPost, tc.host, tc.origin)
		if tc.site != "" {
			req.Header.Set("Sec-Fetch-Site", tc.site)
		}

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d", tc.name, rec.Code, tc.want)
		}

		if got := rec.Header().Get("Access-Control-Allow-Origin") != ""; got != tc.cors {
			t.Errorf("%s: CORS headers = %v, want %v", tc.name, got, tc.cors)
		}
	}

	preflight := request(http.MethodOptions, "vision-pc:8085", "http://localhost:5173")
	preflight.Header.Set("Access-Control-Request-Method", "PATCH")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, preflight)

	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Errorf("preflight = %d %v, want 204 with allowed headers", rec.Code, rec.Header())
	}

	p.SetStrict(true)

	req := request(http.MethodPost, "vision-pc:8085", "http://192.168.1.20:3000")
	req.Header.Set("Sec-Fetch-Site", "cross-site")

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("competition: LAN UI write = %d, want 403", rec.Code)
	}
}
