// Package origins decides which web pages may use the host's API from a
// browser: its own, and any explicitly trusted origin (another UI, a dev
// server). Clients outside a browser send no Origin and are unaffected.
package origins

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Policy is the same-origin rule plus a list of trusted origins.
type Policy struct {
	trusted    map[string]bool
	protection *http.CrossOriginProtection
}

// New trusts each origin in trusted, written scheme://host[:port], e.g.
// "http://localhost:5173".
func New(trusted ...string) (*Policy, error) {
	p := &Policy{trusted: map[string]bool{}, protection: http.NewCrossOriginProtection()}

	for _, origin := range trusted {
		u, err := url.Parse(origin)
		if err != nil || u.Scheme == "" || u.Host == "" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" {
			return nil, fmt.Errorf("origin %q: want scheme://host[:port]", origin)
		}

		normalized := strings.ToLower(u.Scheme + "://" + u.Host)
		if err := p.protection.AddTrustedOrigin(normalized); err != nil {
			return nil, fmt.Errorf("origin %q: %w", origin, err)
		}

		p.trusted[normalized] = true
	}

	return p, nil
}

// Allowed reports whether r may proceed: no Origin (not a browser), the
// host's own origin, or a trusted one. For WebSocket upgrades, which
// cross-origin protection doesn't cover.
func (p *Policy) Allowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}

	if u, err := url.Parse(origin); err == nil && strings.EqualFold(u.Host, r.Host) {
		return true
	}

	return p.trusted[strings.ToLower(origin)]
}

// Handler wraps next: state-changing requests a browser sends on behalf of an
// untrusted site get deny; a trusted origin gets the CORS headers that let
// its page read responses, and its preflights are answered here.
func (p *Policy) Handler(next http.Handler, deny http.Handler) http.Handler {
	p.protection.SetDenyHandler(deny)
	protected := p.protection.Handler(next)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		if p.trusted[strings.ToLower(origin)] {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Add("Vary", "Origin")
			h.Set("Access-Control-Expose-Headers", "ETag")

			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE")
				h.Set("Access-Control-Allow-Headers", "Content-Type, If-Match")
				h.Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)

				return
			}
		}

		protected.ServeHTTP(w, r)
	})
}
