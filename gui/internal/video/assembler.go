// Package video bridges a vision_processor's H.264 RTP stream to browsers:
// it reassembles frames from the multicast RTP packets, repackages them as
// fragmented MP4 without transcoding, and fans the result out to WebSocket
// viewers, who play it through Media Source Extensions.
package video

import (
	"encoding/binary"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/pion/rtp"
)

// RFC 6184 packet types beyond single NAL units (1-23).
const (
	naluSTAPA = 24
	naluFUA   = 28
)

// assembler turns RTP H.264 packets (RFC 6184 single NAL units, STAP-A, and
// FU-A) into access units: the NAL units of one frame. There's no SDP to say
// otherwise, so it assumes packetization mode 1, FFmpeg's default.
//
// A lost packet ruins the frame it belongs to, and every frame after it until
// the next keyframe, since they're predicted from it. So on a sequence gap it
// drops frames until a keyframe arrives rather than passing on a corrupt
// picture. A new SSRC means the encoder restarted (vision_processor does this
// whenever the streamed view changes size), which starts over the same way.
type assembler struct {
	started   bool
	ssrc      uint32
	seq       uint16
	timestamp uint32

	nalus    [][]byte
	size     int    // bytes in nalus and fragment
	fragment []byte // an FU-A NAL unit in progress, nil if none
	broken   bool   // this frame lost a packet
	needKey  bool   // dropping until a keyframe
}

// push takes one packet and returns any frames it completed: the previous
// frame when this packet starts a new one, and this one if it carries the
// marker bit. Payload bytes are copied; pkt may reuse its buffer.
func (a *assembler) push(pkt *rtp.Packet) [][][]byte {
	var out [][][]byte

	if !a.started || pkt.SSRC != a.ssrc {
		*a = assembler{started: true, ssrc: pkt.SSRC, timestamp: pkt.Timestamp, needKey: true}
	} else {
		gap := pkt.SequenceNumber != a.seq+1
		if gap {
			a.broken = true
			a.needKey = true
		}

		if pkt.Timestamp != a.timestamp {
			// The marker of the previous frame never came; it ended anyway.
			out = a.finish(out)
			a.timestamp = pkt.Timestamp

			// The lost packets may have been this frame's first ones rather
			// than the last frame's tail; there's no telling which.
			a.broken = gap
		}
	}

	a.seq = pkt.SequenceNumber

	a.size += len(pkt.Payload)

	if a.size > h264.MaxAccessUnitSize || !a.depacketize(pkt.Payload) {
		a.broken = true
		a.needKey = true
	}

	if pkt.Marker {
		out = a.finish(out)
	}

	return out
}

// depacketize appends pkt's NAL units to the current frame. false means the
// payload was malformed or out of place.
func (a *assembler) depacketize(payload []byte) bool {
	if len(payload) == 0 {
		return false
	}

	switch typ := payload[0] & 0x1f; {
	case typ >= 1 && typ <= 23:
		a.nalus = append(a.nalus, clone(payload))

	case typ == naluSTAPA:
		for rest := payload[1:]; len(rest) > 0; {
			if len(rest) < 2 {
				return false
			}

			size := int(binary.BigEndian.Uint16(rest))
			if len(rest) < 2+size {
				return false
			}

			a.nalus = append(a.nalus, clone(rest[2:2+size]))
			rest = rest[2+size:]
		}

	case typ == naluFUA:
		if len(payload) < 2 {
			return false
		}

		start, end := payload[1]&0x80 != 0, payload[1]&0x40 != 0

		if start {
			// The reconstructed header: the indicator's NRI bits, the
			// fragmented unit's own type.
			a.fragment = append(a.fragment[:0], payload[0]&0xe0|payload[1]&0x1f)
		} else if a.fragment == nil {
			return false // the start fragment was lost
		}

		a.fragment = append(a.fragment, payload[2:]...)

		if end {
			a.nalus = append(a.nalus, a.fragment)
			a.fragment = nil
		}

	default:
		return false
	}

	return true
}

// finish ends the current frame, appending it to out unless it's broken or
// still waiting for a keyframe.
func (a *assembler) finish(out [][][]byte) [][][]byte {
	au, ok := a.nalus, !a.broken && a.fragment == nil && len(a.nalus) > 0
	a.nalus, a.fragment, a.broken, a.size = nil, nil, false, 0

	if !ok {
		return out
	}

	if a.needKey {
		if !h264.IsRandomAccess(au) {
			return out
		}

		a.needKey = false
	}

	return append(out, au)
}

func clone(b []byte) []byte {
	return append([]byte(nil), b...)
}
