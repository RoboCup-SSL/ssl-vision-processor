package multicast

import (
	"log/slog"
	"net"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/gamecontroller"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
	"google.golang.org/protobuf/proto"
)

// VisionConsumer hands the geometry from every wrapper packet to absorb, and
// each detection frame and its sender to track. Only packets carrying
// a detection frame count as heard: those come from vision processors, while
// geometry-only packets include the host's own, looped back (multicast
// loopback is on by default; absorb is idempotent, so that's harmless rather
// than special-cased).
func VisionConsumer(absorb func(*vision.SSL_GeometryData), track func(detection *vision.SSL_DetectionFrame, from net.IP)) Consumer {
	return func(data []byte, from *net.UDPAddr) bool {
		var packet vision.SSL_WrapperPacket
		if err := proto.Unmarshal(data, &packet); err != nil {
			slog.Warn("dropping malformed wrapper packet", "err", err)

			return false
		}

		if geometry := packet.GetGeometry(); geometry != nil {
			absorb(geometry)
		}

		detection := packet.GetDetection()
		if detection != nil && from != nil && track != nil {
			track(detection, from.IP)
		}

		return detection != nil
	}
}

// RefereeConsumer counts valid referee messages and hands each one's yellow
// and blue team names to track. Hearing them is what shows the game
// controller address is right.
func RefereeConsumer(track func(yellow, blue string)) Consumer {
	return func(data []byte, _ *net.UDPAddr) bool {
		var referee gamecontroller.Referee
		if proto.Unmarshal(data, &referee) != nil {
			return false
		}

		if track != nil {
			track(referee.GetYellow().GetName(), referee.GetBlue().GetName())
		}

		return true
	}
}

// TrackerConsumer counts valid TrackerWrapperPacket messages, from any
// autoref, and hands each one and its sender to track.
func TrackerConsumer(track func(packet *vision.TrackerWrapperPacket, from net.IP)) Consumer {
	return func(data []byte, from *net.UDPAddr) bool {
		var packet vision.TrackerWrapperPacket
		if proto.Unmarshal(data, &packet) != nil {
			return false
		}

		if track != nil && from != nil {
			track(&packet, from.IP)
		}

		return true
	}
}
