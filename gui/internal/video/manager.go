package video

import (
	"errors"
	"sync"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/multicast"
)

// ErrUnknownCamera is returned for a camera vision.yml doesn't have.
var ErrUnknownCamera = errors.New("unknown camera")

// Stream is one camera's stream settings, from config.Document.Stream.
type Stream struct {
	Active  bool
	Address string
}

// Manager owns one source per camera that has ever been watched.
type Manager struct {
	verbose bool

	mu      sync.Mutex
	streams map[int]Stream
	ifaces  []multicast.Interface
	sources map[int]*source
}

// NewManager returns a Manager with no cameras until Configure.
func NewManager(verbose bool) *Manager {
	return &Manager{verbose: verbose, sources: map[int]*source{}}
}

// Configure applies the cameras' current stream settings and the host's
// interface selection. Called on every config change and once a second, like
// the host's other sockets; a source's socket only moves if something it
// uses changed.
func (m *Manager) Configure(streams map[int]Stream, ifaces []multicast.Interface) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.streams, m.ifaces = streams, ifaces

	for id, src := range m.sources {
		if st, ok := streams[id]; ok {
			src.configure(st.Active, st.Address, ifaces)
		}
	}
}

// Subscribe starts a viewer of camera id's stream. Call the returned
// function exactly once when done; the channel closes then.
func (m *Manager) Subscribe(id int, mode Mode) (<-chan Message, func(), error) {
	m.mu.Lock()

	st, ok := m.streams[id]
	if !ok {
		m.mu.Unlock()

		return nil, nil, ErrUnknownCamera
	}

	src := m.sources[id]
	if src == nil {
		src = newSource(id, m.verbose)
		m.sources[id] = src
	}

	ifaces := m.ifaces
	m.mu.Unlock()

	src.configure(st.Active, st.Address, ifaces)

	ch, unsubscribe := src.subscribe(mode)

	return ch, unsubscribe, nil
}
