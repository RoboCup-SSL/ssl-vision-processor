package video

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/multicast"
	"github.com/pion/rtp"
)

// Mode is what a viewer receives.
type Mode string

const (
	// ModeFull is every frame.
	ModeFull Mode = "full"
	// ModeKeyframes is keyframes only, about one a second: a fraction of the
	// bandwidth and decode work, for grids of many cameras.
	ModeKeyframes Mode = "keyframes"
)

// Message is one WebSocket message to a viewer: JSON (a format or status
// message) or a binary fMP4 segment.
type Message struct {
	Binary bool
	Data   []byte
}

const (
	// queueSize bounds each viewer's backlog, about 3 s at full rate.
	queueSize = 90
	// idleGrace keeps a source's socket open briefly after its last viewer
	// leaves, so switching views or reloading the page doesn't pay for a
	// fresh join and the wait for a keyframe.
	idleGrace = 5 * time.Second
	// maxGOP bounds the cache of frames since the last keyframe, in case a
	// stream stops sending keyframes.
	maxGOP = 300
	// readBuffer is the kernel receive buffer per stream: room for a
	// keyframe's burst of packets.
	readBuffer = 4 << 20
	// statusInterval is how often viewers get a status message.
	statusInterval = time.Second
)

// endpoint is the part of multicast.Endpoint a source uses, swapped out in
// tests.
type endpoint interface {
	Run(ctx context.Context) error
	SetAddress(address string)
	SetInterfaces(ifaces []multicast.Interface)
	Status(now time.Time) multicast.Status
}

type viewer struct {
	mode    Mode
	ch      chan Message
	lagging bool
}

// source is one camera's stream: its socket, open while anyone watches, and
// everything between packets and viewers.
type source struct {
	id          int
	newEndpoint func(address string, ifaces []multicast.Interface, consume multicast.Consumer) endpoint

	mu      sync.Mutex
	active  bool
	address string
	ifaces  []multicast.Interface

	viewers  map[*viewer]struct{}
	endpoint endpoint           // nil while stopped
	cancel   context.CancelFunc // stops endpoint
	idle     *time.Timer

	asm      assembler
	mux      *muxer
	format   *Format
	init     []byte
	gop      [][]byte // full rate segments since the last keyframe
	keyframe []byte   // the latest keyframes-only segment
	warned   bool
}

func newSource(id int, verbose bool) *source {
	return &source{
		id:      id,
		viewers: map[*viewer]struct{}{},
		mux:     newMuxer(),
		newEndpoint: func(address string, ifaces []multicast.Interface, consume multicast.Consumer) endpoint {
			opts := multicast.Options{Verbose: verbose, ReadBuffer: readBuffer}

			return multicast.NewEndpoint("video", address, ifaces, opts, consume)
		},
	}
}

// configure applies the camera's current stream settings and the host's
// interface selection. A running socket moves if either changed.
func (s *source) configure(active bool, address string, ifaces []multicast.Interface) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.active, s.address, s.ifaces = active, address, ifaces

	if s.endpoint != nil {
		s.endpoint.SetAddress(address)
		s.endpoint.SetInterfaces(ifaces)
	}
}

// subscribe adds a viewer and opens the socket if it's the first. The viewer
// starts with the current format and, so the picture appears at once rather
// than at the next keyframe, the cached frames since the last one.
func (s *source) subscribe(mode Mode) (<-chan Message, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v := &viewer{mode: mode, ch: make(chan Message, queueSize)}
	s.viewers[v] = struct{}{}

	if s.idle != nil {
		s.idle.Stop()
		s.idle = nil
	}

	if s.endpoint == nil {
		s.start()
	}

	s.sendStatus(v)

	if s.init != nil {
		cached := s.gop
		if mode == ModeKeyframes {
			cached = nil
			if s.keyframe != nil {
				cached = [][]byte{s.keyframe}
			}
		}

		if len(cached) > 0 && free(v) >= len(cached)+2 {
			s.sendFormat(v)

			for _, seg := range cached {
				v.ch <- Message{Binary: true, Data: seg}
			}
		} else {
			v.lagging = true
		}
	} else {
		v.lagging = true
	}

	var once sync.Once

	return v.ch, func() { once.Do(func() { s.unsubscribe(v) }) }
}

func (s *source) unsubscribe(v *viewer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.viewers, v)
	close(v.ch)

	if len(s.viewers) == 0 && s.endpoint != nil {
		s.idle = time.AfterFunc(idleGrace, s.stopIfIdle)
	}
}

// start opens the socket. Called with mu held.
func (s *source) start() {
	ctx, cancel := context.WithCancel(context.Background())

	ep := s.newEndpoint(s.address, s.ifaces, s.handle)
	s.endpoint, s.cancel = ep, cancel

	go func() {
		if err := ep.Run(ctx); err != nil && ctx.Err() == nil {
			slog.Error("video stream stopped", "camera", s.id, "err", err)
		}
	}()

	go s.statusLoop(ctx)

	slog.Info("video stream opened", "camera", s.id, "address", s.address)
}

func (s *source) stopIfIdle() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.viewers) > 0 || s.endpoint == nil {
		return
	}

	s.cancel()
	s.endpoint, s.cancel, s.idle = nil, nil, nil

	// The next viewer waits for a fresh keyframe rather than seeing a stale
	// cache.
	s.asm = assembler{}
	s.mux = newMuxer()
	s.format, s.init, s.gop, s.keyframe = nil, nil, nil, nil

	slog.Info("video stream closed, no viewers", "camera", s.id)
}

// handle is the socket's consumer: one RTP packet in, any finished frames
// out to viewers. It reports whether the datagram was RTP, which is what the
// endpoint's status counts.
func (s *source) handle(data []byte) bool {
	var pkt rtp.Packet
	if err := pkt.Unmarshal(data); err != nil || pkt.Version != 2 {
		return false
	}

	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.endpoint == nil {
		return true // a straggler from a socket being closed
	}

	for _, au := range s.asm.push(&pkt) {
		out, err := s.mux.push(au, now)
		if err != nil {
			if !s.warned {
				slog.Warn("dropping video frame", "camera", s.id, "err", err)
				s.warned = true
			}

			continue
		}

		s.publish(out)
	}

	return true
}

// publish caches a frame's output and delivers it. Called with mu held.
func (s *source) publish(out output) {
	if out.init != nil {
		s.init, s.format = out.init, &out.format
		s.gop = nil

		slog.Info("video format", "camera", s.id, "codec", out.format.Codec, "width", out.format.Width, "height", out.format.Height)
	}

	if out.full != nil {
		if out.keyframe != nil || len(s.gop) >= maxGOP {
			s.gop = nil
		}

		s.gop = append(s.gop, out.full)
	}

	if out.keyframe != nil {
		s.keyframe = out.keyframe
	}

	for v := range s.viewers {
		if out.init != nil && !v.lagging {
			if !s.sendFormat(v) {
				v.lagging = true
			}
		}

		seg := out.full
		if v.mode == ModeKeyframes {
			seg = out.keyframe
		}

		if seg != nil {
			s.deliver(v, seg, out.keyframe != nil)
		}
	}
}

// deliver sends one segment. A viewer that falls behind (a full queue) skips
// frames until the next keyframe, since a frame decoded without the ones
// before it is garbage, then resumes with the format and that keyframe.
func (s *source) deliver(v *viewer, seg []byte, key bool) {
	if v.lagging {
		if !key || free(v) < 3 {
			return
		}

		s.sendFormat(v)
		v.lagging = false
	}

	if !trySend(v, Message{Binary: true, Data: seg}) {
		v.lagging = true
	}
}

// sendFormat sends the format message and init segment.
func (s *source) sendFormat(v *viewer) bool {
	if s.format == nil || free(v) < 2 {
		return false
	}

	data, _ := json.Marshal(struct {
		Type string `json:"type"`
		Format
	}{"format", *s.format})

	v.ch <- Message{Data: data}
	v.ch <- Message{Binary: true, Data: s.init}

	return true
}

// statusLoop tells every viewer about the stream once a second, so "waiting
// for the stream" can be told from "the stream isn't arriving".
func (s *source) statusLoop(ctx context.Context) {
	ticker := time.NewTicker(statusInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			for v := range s.viewers {
				s.sendStatus(v)
			}
			s.mu.Unlock()
		}
	}
}

// Status is what a viewer is told about its stream.
type Status struct {
	Type      string `json:"type"`
	Camera    int    `json:"camera"`
	Address   string `json:"address"`
	Active    bool   `json:"active"`
	Receiving bool   `json:"receiving"`
	Packets   uint64 `json:"packets"`
	Source    string `json:"source,omitempty"`
}

// sendStatus is best effort: a full queue just misses one. Called with mu
// held.
func (s *source) sendStatus(v *viewer) {
	status := Status{Type: "status", Camera: s.id, Address: s.address, Active: s.active}

	if s.endpoint != nil {
		ep := s.endpoint.Status(time.Now())
		status.Receiving, status.Packets, status.Source = ep.Receiving, ep.Heard, ep.Source
	}

	data, _ := json.Marshal(status)
	trySend(v, Message{Data: data})
}

func trySend(v *viewer, m Message) bool {
	select {
	case v.ch <- m:
		return true
	default:
		return false
	}
}

func free(v *viewer) int {
	return cap(v.ch) - len(v.ch)
}
