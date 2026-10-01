package multicast

import (
	"testing"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
	"google.golang.org/protobuf/proto"
)

// handleDatagram is exercised directly rather than through Run/Start, so these
// tests need no real socket -- useful in sandboxes where multicast join may be
// blocked.

func TestHandleDatagramCallsAbsorbForGeometry(t *testing.T) {
	b := New("224.5.23.2:10006", nil, false)

	// SSL_GeometryData.field is a required proto2 field, so Marshal needs a
	// complete SSL_GeometryFieldSize even though calib is what this test cares
	// about; calib itself has 12 required fields per entry and is left empty.
	packet := &vision.SSL_WrapperPacket{
		Geometry: &vision.SSL_GeometryData{
			Field: &vision.SSL_GeometryFieldSize{
				FieldLength:   proto.Int32(9000),
				FieldWidth:    proto.Int32(6000),
				GoalWidth:     proto.Int32(1000),
				GoalDepth:     proto.Int32(180),
				BoundaryWidth: proto.Int32(300),
			},
		},
	}

	data, err := proto.Marshal(packet)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got *vision.SSL_GeometryData
	b.handleDatagram(data, func(g *vision.SSL_GeometryData) { got = g })

	if got == nil || got.GetField().GetFieldLength() != 9000 {
		t.Fatalf("absorb called with %v, want field_length 9000", got)
	}
}

func TestHandleDatagramIgnoresPacketWithoutGeometry(t *testing.T) {
	b := New("224.5.23.2:10006", nil, false)

	data, err := proto.Marshal(&vision.SSL_WrapperPacket{})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	called := false
	b.handleDatagram(data, func(*vision.SSL_GeometryData) { called = true })

	if called {
		t.Fatal("absorb called for a packet with no geometry")
	}
}

func TestHandleDatagramDropsMalformedData(t *testing.T) {
	b := New("224.5.23.2:10006", nil, false)

	called := false
	b.handleDatagram([]byte("not a protobuf message"), func(*vision.SSL_GeometryData) { called = true })

	if called {
		t.Fatal("absorb called for malformed data")
	}
}
