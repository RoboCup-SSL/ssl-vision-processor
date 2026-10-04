package multicast

import (
	"log/slog"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/gamecontroller"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
	"google.golang.org/protobuf/proto"
)

// VisionConsumer hands the geometry from every wrapper packet to absorb. Only
// packets carrying a detection frame count as heard: those come from vision
// processors, while geometry-only packets include the host's own, looped back
// (multicast loopback is on by default; absorb is idempotent, so that's
// harmless rather than special-cased).
func VisionConsumer(absorb func(*vision.SSL_GeometryData)) Consumer {
	return func(data []byte) bool {
		var packet vision.SSL_WrapperPacket
		if err := proto.Unmarshal(data, &packet); err != nil {
			slog.Warn("dropping malformed wrapper packet", "err", err)

			return false
		}

		if geometry := packet.GetGeometry(); geometry != nil {
			absorb(geometry)
		}

		return packet.GetDetection() != nil
	}
}

// RefereeConsumer counts valid referee messages. The host doesn't use their
// contents yet; hearing them is what shows the game controller address is
// right.
func RefereeConsumer() Consumer {
	return func(data []byte) bool {
		var referee gamecontroller.Referee

		return proto.Unmarshal(data, &referee) == nil
	}
}
