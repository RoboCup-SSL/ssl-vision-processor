package peers

import (
	"log/slog"
	"net"
	"slices"
	"strings"
	"sync"
	"time"
)

// What a Watch watches.
const (
	KindCalibration = "calibration" // a camera's calibration, from vision_processors
	KindDetections  = "detections"  // a camera's detection frames
	KindGeometry    = "geometry"    // the field geometry this host publishes
	KindReferee     = "referee"     // game controller referee messages
	KindVideo       = "video"       // a camera's RTP stream
)

// Options configures a Watch.
type Options struct {
	Kind string
	// Window is how recently a sender must have been heard to count.
	Window time.Duration
	// MinCount is how many packets within Window a sender needs to count,
	// to tell a steady publisher from a one-off; 0 means 1.
	MinCount int
	// WithSelf marks something this host sends too, so a single other
	// sender is already a conflict.
	WithSelf bool
}

// Conflict is something claimed by more senders than it should be.
type Conflict struct {
	Kind    string `json:"kind" enum:"calibration,detections,geometry,referee,video"`
	Subject string `json:"subject"`
	// CameraID is the camera the subject is about, if any.
	CameraID *int     `json:"cameraId,omitempty"`
	Senders  []Sender `json:"senders"`
}

// Watch tracks who sends each subject (one camera's calibration, say) and
// logs when a second sender appears and when it's gone again. Safe for
// concurrent use.
type Watch struct {
	opts  Options
	names *Names

	mu       sync.Mutex
	subjects map[string]*subject
}

type subject struct {
	cameraID *int
	senders  map[string]*heard
	conflict bool
}

type heard struct {
	from  *net.UDPAddr
	times []time.Time // within Window, newest last
	last  time.Time
}

// NewWatch returns a Watch naming senders through names.
func NewWatch(opts Options, names *Names) *Watch {
	opts.MinCount = max(opts.MinCount, 1)

	return &Watch{opts: opts, names: names, subjects: map[string]*subject{}}
}

// forget is how long a silent sender is kept.
func (w *Watch) forget() time.Duration {
	return max(10*w.opts.Window, 30*time.Second)
}

// Record notes that from sent name (about cameraID, if not nil) at now.
func (w *Watch) Record(name string, cameraID *int, from *net.UDPAddr, now time.Time) {
	if from == nil {
		return
	}

	w.mu.Lock()

	sub := w.subjects[name]
	if sub == nil {
		sub = &subject{cameraID: cameraID, senders: map[string]*heard{}}
		w.subjects[name] = sub
	}

	key := from.String()
	h := sub.senders[key]
	isNew := h == nil

	if isNew {
		h = &heard{from: from}
		sub.senders[key] = h
	}

	h.times = append(w.trim(h.times, now), now)
	h.last = now

	active, changed := w.update(sub, now)
	others := len(sub.senders) - 1
	conflict := sub.conflict
	w.mu.Unlock()

	if isNew {
		go w.logNew(name, from, others)
	}

	if changed {
		go w.logChange(name, conflict, active)
	}
}

// Conflicts lists every current conflict, logging any that ended since the
// last look. Hostnames not yet resolved are left out rather than waited for.
func (w *Watch) Conflicts(now time.Time) []Conflict {
	type found struct {
		name     string
		cameraID *int
		active   []*net.UDPAddr
	}

	var (
		current []found
		ended   []string
	)

	w.mu.Lock()

	for name, sub := range w.subjects {
		for key, h := range sub.senders {
			if now.Sub(h.last) > w.forget() {
				delete(sub.senders, key)
			}
		}

		active, changed := w.update(sub, now)

		if changed && !sub.conflict {
			ended = append(ended, name)
		}

		if sub.conflict {
			current = append(current, found{name, sub.cameraID, active})
		}

		if len(sub.senders) == 0 {
			delete(w.subjects, name)
		}
	}

	w.mu.Unlock()

	for _, name := range ended {
		go w.logChange(name, false, nil)
	}

	out := []Conflict{}

	for _, f := range current {
		senders := make([]Sender, len(f.active))
		for i, a := range f.active {
			senders[i] = w.names.describeCached(a)
		}

		out = append(out, Conflict{Kind: w.opts.Kind, Subject: f.name, CameraID: f.cameraID, Senders: senders})
	}

	slices.SortFunc(out, func(a, b Conflict) int { return strings.Compare(a.Subject, b.Subject) })

	return out
}

// update recomputes sub's conflict, returning its active senders (by address)
// and whether the conflict started or ended. Callers must hold mu.
func (w *Watch) update(sub *subject, now time.Time) ([]*net.UDPAddr, bool) {
	var active []*net.UDPAddr

	for _, h := range sub.senders {
		h.times = w.trim(h.times, now)
		if len(h.times) >= w.opts.MinCount {
			active = append(active, h.from)
		}
	}

	slices.SortFunc(active, func(a, b *net.UDPAddr) int { return strings.Compare(a.String(), b.String()) })

	limit := 1
	if w.opts.WithSelf {
		limit = 0
	}

	conflict := len(active) > limit
	changed := conflict != sub.conflict
	sub.conflict = conflict

	return active, changed
}

func (w *Watch) trim(times []time.Time, now time.Time) []time.Time {
	i := 0
	for i < len(times) && now.Sub(times[i]) > w.opts.Window {
		i++
	}

	return times[i:]
}

func (w *Watch) logNew(name string, from *net.UDPAddr, others int) {
	s := w.names.Describe(from)
	slog.Info("new sender", "kind", w.opts.Kind, "subject", name, "from", s.Address, "host", s.Host, "local", s.Local, "otherSenders", others)
}

func (w *Watch) logChange(name string, conflict bool, active []*net.UDPAddr) {
	if !conflict {
		slog.Info("sender conflict resolved", "kind", w.opts.Kind, "subject", name)

		return
	}

	senders := make([]string, len(active))
	for i, a := range active {
		senders[i] = w.names.Describe(a).String()
	}

	if w.opts.WithSelf {
		slog.Warn("another host is sending what this host sends", "kind", w.opts.Kind, "subject", name, "senders", senders)

		return
	}

	slog.Warn("several senders claim the same thing", "kind", w.opts.Kind, "subject", name, "count", len(senders), "senders", senders)
}
