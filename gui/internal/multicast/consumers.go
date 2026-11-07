package multicast

import (
	"log/slog"
	"net"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/gamecontroller"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
	"google.golang.org/protobuf/proto"
)

// VisionConsumer hands the geometry from every wrapper packet to absorb, each
// detection frame to track, and each vision_processor announce to announce,
// each with its sender. The host's own looped-back packets are dropped. Only
// packets carrying a detection frame or an announce count as heard: those come
// from vision processors, and one without a camera only announces.
func VisionConsumer(absorb func(*vision.SSL_GeometryData, *net.UDPAddr), track func(*vision.SSL_DetectionFrame, *net.UDPAddr), announce func(*vision.SSL_VPConfig, *net.UDPAddr)) Consumer {
	return func(data []byte, from *net.UDPAddr) bool {
		if IsOwn(from) {
			return false
		}

		var packet vision.SSL_WrapperPacket
		if err := proto.Unmarshal(data, &packet); err != nil {
			slog.Warn("dropping malformed wrapper packet", "from", from, "err", err)

			return false
		}

		if geometry := packet.GetGeometry(); geometry != nil && absorb != nil {
			absorb(geometry, from)
		}

		detection := packet.GetDetection()
		if detection != nil && from != nil && track != nil {
			track(detection, from)
		}

		config := packet.GetConfig()
		if config != nil && from != nil && announce != nil {
			announce(config, from)
		}

		return detection != nil || config != nil
	}
}

// RefereeConsumer counts valid referee messages and hands each one's yellow
// and blue team names to track. Hearing them is what shows the game
// controller address is right.
func RefereeConsumer(track func(yellow, blue string, from *net.UDPAddr)) Consumer {
	return func(data []byte, from *net.UDPAddr) bool {
		var referee gamecontroller.Referee
		if proto.Unmarshal(data, &referee) != nil {
			return false
		}

		if track != nil {
			track(referee.GetYellow().GetName(), referee.GetBlue().GetName(), from)
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
