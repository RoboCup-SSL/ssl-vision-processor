// Package multicast wires the SSL vision multicast group to the geometry
// service: inbound SSL_WrapperPacket.geometry is handed to an absorb callback,
// and outbound bytes are sent back onto the same group.
package multicast

import (
	"context"
	"log/slog"
	"net"

	"github.com/RoboCup-SSL/ssl-go-tools/pkg/sslnet"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
	"google.golang.org/protobuf/proto"
)

// Bridge pairs a receiver and a sender on one multicast address. Our own
// outbound packets loop back to the receiver (multicast loopback is on by
// default); Absorb is idempotent, so this is harmless rather than special-cased.
type Bridge struct {
	server *sslnet.MulticastServer
	client *sslnet.UdpClient
}

// New prepares a bridge on address (e.g. "224.5.23.2:10006"). skipInterfaces
// excludes named network interfaces from both receive and send.
func New(address string, skipInterfaces []string, verbose bool) *Bridge {
	server := sslnet.NewMulticastServer(address)
	server.SkipInterfaces = skipInterfaces
	server.Verbose = verbose

	client := sslnet.NewUdpClient(address, "")

	return &Bridge{server: server, client: client}
}

// Run starts receiving until ctx is cancelled, calling absorb with the
// geometry from every wrapper packet that carries one.
func (b *Bridge) Run(ctx context.Context, absorb func(*vision.SSL_GeometryData)) error {
	b.server.Consumer = func(data []byte, _ *net.UDPAddr) {
		b.handleDatagram(data, absorb)
	}

	b.server.Start()
	b.client.Start()

	<-ctx.Done()

	b.server.Stop()
	b.client.Stop()

	return ctx.Err()
}

// handleDatagram parses one datagram and, if it carries geometry, calls absorb.
func (b *Bridge) handleDatagram(data []byte, absorb func(*vision.SSL_GeometryData)) {
	var packet vision.SSL_WrapperPacket
	if err := proto.Unmarshal(data, &packet); err != nil {
		slog.Warn("dropping malformed wrapper packet", "err", err)
		return
	}

	if geometry := packet.GetGeometry(); geometry != nil {
		absorb(geometry)
	}
}

// Send publishes data onto the multicast group.
func (b *Bridge) Send(data []byte) {
	b.client.Send(data)
}
