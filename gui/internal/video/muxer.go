package video

import (
	"bytes"
	"fmt"
	"time"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/fmp4"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/fmp4/seekablebuffer"
	mp4codecs "github.com/bluenviron/mediacommon/v2/pkg/formats/mp4/codecs"
)

// timeScale is the fMP4 track's clock rate, the usual one for video.
const timeScale = 90000

// Format describes the stream an init segment starts: what a browser needs
// to check it can play it before trying.
type Format struct {
	// Codec is the RFC 6381 codec string, e.g. "avc1.4d4028", for
	// MediaSource.isTypeSupported.
	Codec  string `json:"codec"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// output is what one frame produces: a new init segment if the stream's
// parameters changed, the frame as a media segment for full rate viewers,
// and, for a keyframe, a second media segment for keyframes-only viewers.
// Each timeline needs its own segment, since durations and sequence numbers
// differ.
type output struct {
	init     []byte
	format   Format
	full     []byte
	keyframe []byte
}

// muxer packages frames as fragmented MP4, one frame per segment, so a
// segment is ready the moment its frame is. vision_processor stamps frames
// at a nominal 30 fps whatever rate it really runs at, so timing comes from
// arrival time instead.
type muxer struct {
	sps, pps []byte

	full     timeline
	keyframe timeline
}

// timeline is one output's clock. Durations are estimates, the gap before
// the previous frame, so a frame isn't held back waiting for the next one;
// the browser appends in sequence mode, where only durations matter.
type timeline struct {
	seq     uint32
	base    uint64
	last    time.Time
	minimum uint64
	initial uint64
	maximum uint64
}

func newMuxer() *muxer {
	return &muxer{
		full:     timeline{minimum: timeScale / 200, initial: timeScale / 30, maximum: timeScale},
		keyframe: timeline{minimum: timeScale / 30, initial: timeScale, maximum: 5 * timeScale},
	}
}

// push packages one frame (from assembler) that arrived at now.
func (m *muxer) push(au [][]byte, now time.Time) (output, error) {
	var out output

	sps, pps := m.sps, m.pps

	var frame [][]byte

	for _, nalu := range au {
		switch h264.NALUType(nalu[0] & 0x1f) {
		case h264.NALUTypeSPS:
			sps = nalu
		case h264.NALUTypePPS:
			pps = nalu
		case h264.NALUTypeAccessUnitDelimiter:
		default:
			// Parameter sets live in the init segment; delimiters are noise.
			frame = append(frame, nalu)
		}
	}

	if sps == nil || pps == nil {
		return out, fmt.Errorf("no SPS/PPS yet")
	}

	if !bytes.Equal(sps, m.sps) || !bytes.Equal(pps, m.pps) {
		init, format, err := initSegment(sps, pps)
		if err != nil {
			return out, err
		}

		m.sps, m.pps = sps, pps
		out.init, out.format = init, format
	}

	if len(frame) == 0 {
		return out, nil
	}

	payload, err := h264.AVCC(frame).Marshal()
	if err != nil {
		return out, err
	}

	key := h264.IsRandomAccess(au)

	if out.full, err = m.full.segment(payload, key, now); err != nil {
		return out, err
	}

	if key {
		if out.keyframe, err = m.keyframe.segment(payload, true, now); err != nil {
			return out, err
		}
	}

	return out, nil
}

func (t *timeline) segment(payload []byte, key bool, now time.Time) ([]byte, error) {
	duration := t.initial
	if !t.last.IsZero() {
		duration = uint64(now.Sub(t.last).Seconds() * timeScale)
		duration = min(max(duration, t.minimum), t.maximum)
	}

	t.last = now
	t.seq++

	part := fmp4.Part{
		SequenceNumber: t.seq,
		Tracks: []*fmp4.PartTrack{{
			ID:       1,
			BaseTime: t.base,
			Samples: []*fmp4.Sample{{
				Duration:        uint32(duration),
				IsNonSyncSample: !key,
				Payload:         payload,
			}},
		}},
	}

	t.base += duration

	var buf seekablebuffer.Buffer
	if err := part.Marshal(&buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func initSegment(sps, pps []byte) ([]byte, Format, error) {
	var parsed h264.SPS
	if err := parsed.Unmarshal(sps); err != nil {
		return nil, Format{}, fmt.Errorf("parsing SPS: %w", err)
	}

	if len(sps) < 4 {
		return nil, Format{}, fmt.Errorf("SPS too short")
	}

	init := fmp4.Init{
		Tracks: []*fmp4.InitTrack{{
			ID:        1,
			TimeScale: timeScale,
			Codec:     &mp4codecs.H264{SPS: sps, PPS: pps},
		}},
	}

	var buf seekablebuffer.Buffer
	if err := init.Marshal(&buf); err != nil {
		return nil, Format{}, err
	}

	return buf.Bytes(), Format{
		Codec:  fmt.Sprintf("avc1.%02x%02x%02x", sps[1], sps[2], sps[3]),
		Width:  parsed.Width(),
		Height: parsed.Height(),
	}, nil
}
