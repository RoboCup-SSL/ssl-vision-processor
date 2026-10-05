package video

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/multicast"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/fmp4"
	"github.com/pion/rtp"
)

// The clips are x264 output from ffmpeg's test pattern: 320x240 has 20
// frames with a keyframe every 10, 160x120 has 10 with one keyframe.

// loadClip splits an Annex B file into access units: a new one starts at a
// parameter set or delimiter after a slice, or at a slice whose first
// macroblock is 0 when the current unit already has one.
func loadClip(t *testing.T, name string) [][][]byte {
	t.Helper()

	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}

	nalus := splitAnnexB(data)

	var aus [][][]byte

	var cur [][]byte

	hasSlice := false

	for _, n := range nalus {
		typ := n[0] & 0x1f
		slice := typ == 1 || typ == 5
		firstMB := slice && len(n) > 1 && n[1]&0x80 != 0

		if hasSlice && (!slice || firstMB) {
			aus = append(aus, cur)
			cur, hasSlice = nil, false
		}

		cur = append(cur, n)
		hasSlice = hasSlice || slice
	}

	return append(aus, cur)
}

// splitAnnexB splits a whole Annex B file at its start codes. (h264.AnnexB
// is meant for one frame and caps the NAL unit count.)
func splitAnnexB(data []byte) [][]byte {
	var nalus [][]byte

	start := -1

	for i := 0; i+3 <= len(data); i++ {
		if data[i] != 0 || data[i+1] != 0 || data[i+2] != 1 {
			continue
		}

		if start >= 0 {
			nalus = append(nalus, bytes.TrimRight(data[start:i], "\x00"))
		}

		start = i + 3
		i += 2
	}

	return append(nalus, data[start:])
}

// packetizer is the sending side of RFC 6184, enough to drive the assembler:
// parameter sets go in one STAP-A, small units alone, big ones as FU-A.
type packetizer struct {
	ssrc      uint32
	seq       uint16
	timestamp uint32
}

const mtu = 1000

func (p *packetizer) packets(au [][]byte) []*rtp.Packet {
	var payloads [][]byte

	var params [][]byte

	for _, n := range au {
		if typ := n[0] & 0x1f; typ == 7 || typ == 8 {
			params = append(params, n)

			continue
		}

		if len(params) > 0 {
			stap := []byte{24}
			for _, ps := range params {
				stap = append(stap, byte(len(ps)>>8), byte(len(ps)))
				stap = append(stap, ps...)
			}

			payloads = append(payloads, stap)
			params = nil
		}

		if len(n) <= mtu {
			payloads = append(payloads, n)

			continue
		}

		header, body := n[0], n[1:]
		for i := 0; i < len(body); i += mtu {
			end := min(i+mtu, len(body))

			fu := []byte{header&0xe0 | 28, header & 0x1f}
			if i == 0 {
				fu[1] |= 0x80
			}

			if end == len(body) {
				fu[1] |= 0x40
			}

			payloads = append(payloads, append(fu, body[i:end]...))
		}
	}

	pkts := make([]*rtp.Packet, len(payloads))
	for i, payload := range payloads {
		p.seq++
		pkts[i] = &rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				PayloadType:    96,
				SequenceNumber: p.seq,
				Timestamp:      p.timestamp,
				SSRC:           p.ssrc,
				Marker:         i == len(payloads)-1,
			},
			Payload: payload,
		}
	}

	p.timestamp += 3000

	return pkts
}

func assemble(a *assembler, pkts []*rtp.Packet) [][][]byte {
	var out [][][]byte
	for _, pkt := range pkts {
		out = append(out, a.push(pkt)...)
	}

	return out
}

func TestAssemblerRoundTrip(t *testing.T) {
	clip := loadClip(t, "320x240.h264")
	if len(clip) != 20 {
		t.Fatalf("clip has %d access units, want 20", len(clip))
	}

	p := &packetizer{ssrc: 1}

	var a assembler

	var got [][][]byte
	for _, au := range clip {
		got = append(got, assemble(&a, p.packets(au))...)
	}

	if len(got) != len(clip) {
		t.Fatalf("assembled %d frames, want %d", len(got), len(clip))
	}

	for i := range clip {
		if len(got[i]) != len(clip[i]) {
			t.Fatalf("frame %d: %d NAL units, want %d", i, len(got[i]), len(clip[i]))
		}

		for j := range clip[i] {
			if !bytes.Equal(got[i][j], clip[i][j]) {
				t.Fatalf("frame %d NAL %d differs", i, j)
			}
		}
	}
}

func TestAssemblerDropsUntilKeyframeAfterLoss(t *testing.T) {
	clip := loadClip(t, "320x240.h264")
	p := &packetizer{ssrc: 1}

	var a assembler

	var got [][][]byte

	for i, au := range clip {
		pkts := p.packets(au)
		if i == 3 {
			pkts = pkts[1:] // lose the first packet of frame 3
		}

		got = append(got, assemble(&a, pkts)...)
	}

	// Frames 0-2 survive; 3-9 are lost or depend on 3; 10 is the next
	// keyframe and everything after it is fine.
	if len(got) != 3+10 {
		t.Fatalf("got %d frames, want 13", len(got))
	}

	if !h264.IsRandomAccess(got[3]) {
		t.Fatal("first frame after the loss isn't a keyframe")
	}
}

func TestAssemblerRestartsOnNewSSRC(t *testing.T) {
	small := loadClip(t, "160x120.h264")
	big := loadClip(t, "320x240.h264")

	var a assembler

	p := &packetizer{ssrc: 1}
	for _, au := range big[:5] {
		assemble(&a, p.packets(au))
	}

	// The encoder restarted: new SSRC, unrelated sequence numbers.
	p = &packetizer{ssrc: 2, seq: 40000}

	got := assemble(&a, p.packets(small[0]))
	if len(got) != 1 || !h264.IsRandomAccess(got[0]) {
		t.Fatalf("got %d frames after the restart, want the new keyframe", len(got))
	}
}

func TestMuxerFormatsAndKeyframes(t *testing.T) {
	m := newMuxer()
	now := time.Now()

	var inits []Format

	keyframes, full := 0, 0

	for _, clip := range []string{"320x240.h264", "160x120.h264"} {
		for _, au := range loadClip(t, clip) {
			now = now.Add(33 * time.Millisecond)

			out, err := m.push(au, now)
			if err != nil {
				t.Fatal(err)
			}

			if out.init != nil {
				var init fmp4.Init
				if err := init.Unmarshal(bytes.NewReader(out.init)); err != nil {
					t.Fatalf("init segment doesn't parse: %v", err)
				}

				inits = append(inits, out.format)
			}

			if out.full != nil {
				full++

				var parts fmp4.Parts
				if err := parts.Unmarshal(out.full); err != nil {
					t.Fatalf("media segment doesn't parse: %v", err)
				}

				sample := parts[0].Tracks[0].Samples[0]
				if sample.IsNonSyncSample == (out.keyframe != nil) {
					t.Fatal("sync flag doesn't match the keyframe output")
				}
			}

			if out.keyframe != nil {
				keyframes++
			}
		}
	}

	if len(inits) != 2 || inits[0].Width != 320 || inits[0].Height != 240 || inits[1].Width != 160 {
		t.Fatalf("formats = %+v, want 320x240 then 160x120", inits)
	}

	if inits[0].Codec[:5] != "avc1." || len(inits[0].Codec) != 11 {
		t.Errorf("codec = %q, want avc1.PPCCLL", inits[0].Codec)
	}

	if full != 30 || keyframes != 3 {
		t.Fatalf("full=%d keyframes=%d, want 30 and 3", full, keyframes)
	}
}

// fakeEndpoint stands in for the multicast socket.
type fakeEndpoint struct{ address string }

func (f *fakeEndpoint) Run(ctx context.Context) error {
	<-ctx.Done()

	return ctx.Err()
}
func (f *fakeEndpoint) SetAddress(address string)                  { f.address = address }
func (f *fakeEndpoint) Reopen()                                    {}
func (f *fakeEndpoint) SetInterfaces(ifaces []multicast.Interface) {}
func (f *fakeEndpoint) Status(now time.Time) multicast.Status {
	return multicast.Status{Address: f.address, Receiving: true, Heard: 7}
}

func testSource() *source {
	s := newSource(0, false)
	s.newEndpoint = func(address string, _ []multicast.Interface, _ multicast.Consumer) endpoint {
		return &fakeEndpoint{address: address}
	}
	s.configure(true, "224.5.23.100:10100", nil)

	return s
}

// feed sends a clip through the source as raw datagrams.
func feed(t *testing.T, s *source, p *packetizer, clip [][][]byte) {
	t.Helper()

	for _, au := range clip {
		for _, pkt := range p.packets(au) {
			data, err := pkt.Marshal()
			if err != nil {
				t.Fatal(err)
			}

			if !s.handle(data, nil) {
				t.Fatal("source rejected an RTP packet")
			}
		}
	}
}

// drain reads what's queued: the JSON message types and the binary count.
func drain(ch <-chan Message) (types []string, binaries int) {
	for {
		select {
		case m := <-ch:
			if m.Binary {
				binaries++

				continue
			}

			var head struct{ Type string }
			_ = json.Unmarshal(m.Data, &head)
			types = append(types, head.Type)
		default:
			return types, binaries
		}
	}
}

func TestSourceFansOutByMode(t *testing.T) {
	s := testSource()
	clip := loadClip(t, "320x240.h264")

	full, stopFull := s.subscribe(ModeFull)
	defer stopFull()

	keys, stopKeys := s.subscribe(ModeKeyframes)
	defer stopKeys()

	feed(t, s, &packetizer{ssrc: 1}, clip)

	types, n := drain(full)
	if len(types) != 2 || types[0] != "status" || types[1] != "format" || n != 1+20 {
		t.Fatalf("full viewer got %v and %d binaries, want status, format, init + 20 frames", types, n)
	}

	types, n = drain(keys)
	if len(types) != 2 || types[1] != "format" || n != 1+2 {
		t.Fatalf("keyframes viewer got %v and %d binaries, want format, init + 2 keyframes", types, n)
	}
}

func TestSourceLateViewerGetsCachedGOP(t *testing.T) {
	s := testSource()
	clip := loadClip(t, "320x240.h264")

	first, stop := s.subscribe(ModeFull)
	defer stop()

	feed(t, s, &packetizer{ssrc: 1}, clip[:14]) // keyframe at 10, then 11-13
	drain(first)

	late, stopLate := s.subscribe(ModeFull)
	defer stopLate()

	types, n := drain(late)
	if len(types) != 2 || types[1] != "format" || n != 1+4 {
		t.Fatalf("late viewer got %v and %d binaries, want format, init + frames 10-13", types, n)
	}
}

func TestSourceSlowViewerResumesAtKeyframe(t *testing.T) {
	s := testSource()
	clip := loadClip(t, "320x240.h264")
	p := &packetizer{ssrc: 1}

	ch, stop := s.subscribe(ModeFull)
	defer stop()

	// 100 frames without reading overflows the queue.
	for range 5 {
		feed(t, s, p, clip)
	}

	drain(ch)

	// The next frames are mid-GOP: still skipped until a keyframe, which
	// comes with a fresh format.
	feed(t, s, p, clip)

	types, n := drain(ch)
	if len(types) != 1 || types[0] != "format" || n != 1+20 {
		t.Fatalf("after catching up got %v and %d binaries, want format, init + 20 frames", types, n)
	}
}

func TestSourceStopsWhenIdleAndForgetsCache(t *testing.T) {
	s := testSource()

	_, stop := s.subscribe(ModeFull)
	feed(t, s, &packetizer{ssrc: 1}, loadClip(t, "320x240.h264"))
	stop()

	s.stopIfIdle()

	if s.endpoint != nil || s.init != nil || s.gop != nil {
		t.Fatal("idle source kept its socket or cache")
	}
}

func TestHandleWebSocketRejectsBadRequests(t *testing.T) {
	m := NewManager(false)
	m.Configure(map[int]Stream{0: {Active: true, Address: "224.5.23.100:10100"}}, nil)

	mux := http.NewServeMux()
	mux.Handle("GET /ws/video/{id}", HandleWebSocket(m))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	for path, want := range map[string]int{
		"/ws/video/7":               http.StatusNotFound,
		"/ws/video/x":               http.StatusBadRequest,
		"/ws/video/0?mode=sideways": http.StatusBadRequest,
	} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}

		_ = resp.Body.Close()

		if resp.StatusCode != want {
			t.Errorf("%s: status %d, want %d", path, resp.StatusCode, want)
		}
	}
}
