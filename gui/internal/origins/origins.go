// Package origins decides which web pages may use the host's API from a
// browser, and which names the host answers to.
//
// Origins: the host's own pages are always allowed. In a lab, so is any page
// served from this network (localhost, a private or link-local IP, a .local
// or single-label name): a public website can't have such an origin, since
// the browser sets Origin from the page's real address. On a competition
// field (SetStrict) only the host's own pages and the trusted list are. Clients
// outside a browser send no Origin and are unaffected.
//
// Hosts: a request must name the host by something it really is (an IP,
// localhost, a .local or single-label name, its own hostname, or an allowed
// alias). That stops DNS rebinding, where an attacker's domain is pointed at
// this host so the browser treats it as same-origin.
package origins

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
)

// Policy is the origin and host rules.
type Policy struct {
	trusted    map[string]bool
	trustAll   bool
	allowAny   bool
	strict     atomic.Bool
	hostsMu    sync.RWMutex
	ownHosts   map[string]bool
	allowHosts map[string]bool
}

// Options configures a Policy.
type Options struct {
	// TrustedOrigins are allowed in both modes, written scheme://host[:port].
	// "*" trusts every origin.
	TrustedOrigins []string
	// AllowedHosts are extra names this host answers to, such as a DNS alias.
	// "*" turns the host check off.
	AllowedHosts []string
}

// New builds a Policy in lab mode.
func New(opts Options) (*Policy, error) {
	p := &Policy{trusted: map[string]bool{}, ownHosts: map[string]bool{}, allowHosts: map[string]bool{}}

	for _, origin := range opts.TrustedOrigins {
		if origin == "*" {
			p.trustAll = true

			continue
		}

		u, err := url.Parse(origin)
		if err != nil || u.Scheme == "" || u.Host == "" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" {
			return nil, fmt.Errorf("origin %q: want scheme://host[:port] or *", origin)
		}

		p.trusted[strings.ToLower(u.Scheme+"://"+u.Host)] = true
	}

	for _, host := range opts.AllowedHosts {
		if host == "*" {
			p.allowAny = true

			continue
		}

		p.allowHosts[strings.ToLower(host)] = true
	}

	return p, nil
}

// SetStrict switches between lab mode (false) and competition mode (true).
func (p *Policy) SetStrict(strict bool) { p.strict.Store(strict) }

// AddOwnHosts records names this machine goes by, such as its hostname and
// the names its addresses reverse-resolve to.
func (p *Policy) AddOwnHosts(names ...string) {
	p.hostsMu.Lock()
	defer p.hostsMu.Unlock()

	for _, name := range names {
		if name = strings.TrimSuffix(strings.ToLower(name), "."); name != "" {
			p.ownHosts[name] = true
		}
	}
}

// HostAllowed reports whether r names this host by something it is.
func (p *Policy) HostAllowed(r *http.Request) bool {
	if p.allowAny {
		return true
	}

	// Rebinding needs a domain name, so any IP literal is safe.
	host := hostname(r.Host)
	if _, err := netip.ParseAddr(host); err == nil || isLocalName(host) {
		return true
	}

	p.hostsMu.RLock()
	defer p.hostsMu.RUnlock()

	return p.ownHosts[host] || p.allowHosts[host]
}

// Allowed reports whether a browser page at r's Origin may use the API: no
// Origin (not a browser), the host's own origin, or a trusted one. For
// WebSocket upgrades as well as Handler.
func (p *Policy) Allowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}

	u, err := url.Parse(origin)
	if err != nil {
		return false
	}

	return strings.EqualFold(u.Host, r.Host) || p.trustedOrigin(u)
}

func (p *Policy) trustedOrigin(u *url.URL) bool {
	if p.trustAll || p.trusted[strings.ToLower(u.Scheme+"://"+u.Host)] {
		return true
	}

	return !p.strict.Load() && isLocalName(hostname(u.Host))
}

// Handler wraps next. A request naming an unknown host gets badHost. A
// state-changing request a browser sends on behalf of an untrusted page gets
// deny, as http.CrossOriginProtection decides it (Sec-Fetch-Site, else
// Origin). A trusted page gets the CORS headers that let it read responses,
// and its preflights are answered here.
func (p *Policy) Handler(next, deny, badHost http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !p.HostAllowed(r) {
			badHost.ServeHTTP(w, r)

			return
		}

		origin := r.Header.Get("Origin")
		u, _ := url.Parse(origin)
		trusted := origin != "" && u != nil && !strings.EqualFold(u.Host, r.Host) && p.trustedOrigin(u)

		if trusted {
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

		if !safeMethod(r.Method) && !trusted && !p.sameOrigin(r) {
			deny.ServeHTTP(w, r)

			return
		}

		next.ServeHTTP(w, r)
	})
}

// sameOrigin is http.CrossOriginProtection's test: Sec-Fetch-Site if the
// browser sent it, else Origin against Host, else not a browser.
func (p *Policy) sameOrigin(r *http.Request) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "same-origin", "none":
		return true
	case "":
	default:
		return false
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}

	u, err := url.Parse(origin)

	return err == nil && strings.EqualFold(u.Host, r.Host)
}

func safeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

// hostname is host without its port, lowercased, without a trailing dot.
func hostname(hostport string) string {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}

	return strings.TrimSuffix(strings.ToLower(strings.Trim(host, "[]")), ".")
}

// isLocalName reports whether host can only mean something on this network:
// localhost, an IP literal that is loopback, private or link-local, a .local
// (mDNS) name, or a single-label name. No public website has one.
func isLocalName(host string) bool {
	if addr, err := netip.ParseAddr(host); err == nil {
		return addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast()
	}

	return host == "localhost" || strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".local") || (host != "" && !strings.Contains(host, "."))
}
