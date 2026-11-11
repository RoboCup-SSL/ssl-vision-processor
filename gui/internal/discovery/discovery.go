// Package discovery keeps the table of vision_processor instances heard
// announcing themselves: the SSL_VPConfig each sends on the vision group about
// once a second, keyed by its instance name.
package discovery

import (
	"cmp"
	"log/slog"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Stale is how long an instance can go unheard and still count as live:
// three missed announces.
const Stale = 3 * time.Second

// Forget drops an instance silent this long, so one that was renamed or shut
// down stops showing.
const Forget = 30 * time.Second

// Config is an SSL_VPConfig as canonical protojson.
type Config []byte

func (c Config) MarshalJSON() ([]byte, error) { return c, nil }

// Instance is one vision_processor, as it last announced itself.
type Instance struct {
	Name     string `json:"instance"`
	CameraID uint32 `json:"cameraId"`
	// Address is the sender's IP and port.
	Address   string    `json:"address"`
	LastHeard time.Time `json:"lastHeard"`
	Live      bool      `json:"live"`
	Config    Config    `json:"config"`
}

type entry struct {
	config  *vision.SSL_VPConfig
	encoded Config
	address string
	last    time.Time
}

// Registry is safe for concurrent use.
type Registry struct {
	mu        sync.Mutex
	instances map[string]*entry
}

func NewRegistry() *Registry {
	return &Registry{instances: map[string]*entry{}}
}

// Record notes config, announced by from at now. An announce without an
// instance name can't be told apart from any other, so it's dropped.
func (r *Registry) Record(config *vision.SSL_VPConfig, from *net.UDPAddr, now time.Time) {
	name := config.GetInstance()
	if name == "" {
		return
	}

	address := from.String()

	r.mu.Lock()
	defer r.mu.Unlock()

	e := r.instances[name]
	if e != nil && e.address == address && proto.Equal(e.config, config) {
		e.last = now

		return
	}

	encoded, err := protojson.Marshal(config)
	if err != nil {
		slog.Warn("dropping vision_processor announce", "instance", name, "from", address, "err", err)

		return
	}

	switch {
	case e == nil:
		slog.Info("vision_processor discovered", "instance", name, "cam", config.GetCameraId(), "from", address)
	case e.address != address:
		slog.Info("vision_processor moved", "instance", name, "from", address, "was", e.address)
	default:
		slog.Info("vision_processor config changed", "instance", name, "cam", config.GetCameraId())
	}

	r.instances[name] = &entry{config: config, encoded: encoded, address: address, last: now}
}

// Instances lists every instance heard within Forget, by camera_id then name.
func (r *Registry) Instances(now time.Time) []Instance {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := []Instance{}

	for name, e := range r.instances {
		if now.Sub(e.last) > Forget {
			slog.Info("vision_processor gone", "instance", name, "from", e.address)
			delete(r.instances, name)

			continue
		}

		out = append(out, Instance{
			Name:      name,
			CameraID:  e.config.GetCameraId(),
			Address:   e.address,
			LastHeard: e.last,
			Live:      now.Sub(e.last) <= Stale,
			Config:    e.encoded,
		})
	}

	slices.SortFunc(out, func(a, b Instance) int {
		return cmp.Or(cmp.Compare(a.CameraID, b.CameraID), cmp.Compare(a.Name, b.Name))
	})

	return out
}
