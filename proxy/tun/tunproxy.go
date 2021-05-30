package tun

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	adapters "github.com/Dreamacro/clash/adapters/inbound"
	"github.com/Dreamacro/clash/transport/socks5"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"
	"github.com/Dreamacro/clash/proxy/tun/dev"
	"github.com/Dreamacro/clash/tunnel"

	"encoding/binary"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

const nicID tcpip.NICID = 1

// tunAdapter is the wraper of tun
type tunAdapter struct {
	device  dev.TunDevice
	ipstack *stack.Stack

	dnsserver *DNSServer
}

// NewTunProxy create TunProxy under Linux OS.
func NewTunProxy(deviceURL string) (TunAdapter, error) {

	var err error

	url, err := url.Parse(deviceURL)
	if err != nil {
		return nil, fmt.Errorf("invalid tun device url: %v", err)
	}

	tundev, err := dev.OpenTunDevice(*url)
	if err != nil {
		return nil, fmt.Errorf("can't open tun: %v", err)
	}

	ipstack := stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol, ipv6.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol},
	})

	tl := &tunAdapter{
		device:  tundev,
		ipstack: ipstack,
	}

	linkEP, err := tundev.AsLinkEndpoint()
	if err != nil {
		return nil, fmt.Errorf("unable to create virtual endpoint: %v", err)
	}

	if err := ipstack.CreateNIC(nicID, linkEP); err != nil {
		return nil, fmt.Errorf("fail to create NIC in ipstack: %v", err)
	}

	ipstack.SetPromiscuousMode(nicID, true) // Accept all the traffice from this NIC
	ipstack.SetSpoofing(nicID, true)        // Otherwise our TCP connection can not find the route backward

	// Add route for ipv4 & ipv6
	// So FindRoute will return correct route to tun NIC
	subnet, _ := tcpip.NewSubnet(tcpip.Address(strings.Repeat("\x00", 4)), tcpip.AddressMask(strings.Repeat("\x00", 4)))
	ipstack.AddRoute(tcpip.Route{Destination: subnet, Gateway: "", NIC: nicID})
	subnet, _ = tcpip.NewSubnet(tcpip.Address(strings.Repeat("\x00", 6)), tcpip.AddressMask(strings.Repeat("\x00", 6)))
	ipstack.AddRoute(tcpip.Route{Destination: subnet, Gateway: "", NIC: nicID})

	// TCP handler
	// maximum number of half-open tcp connection set to 1024
	// receive buffer size set to 20k
	tcpFwd := tcp.NewForwarder(ipstack, 20*1024, 1024, func(r *tcp.ForwarderRequest) {
		var wq waiter.Queue
		ep, err := r.CreateEndpoint(&wq)
		if err != nil {
			log.Warnln("Can't create TCP Endpoint in ipstack: %v", err)
			r.Complete(true)
			return
		}
		r.Complete(false)

		conn := gonet.NewTCPConn(&wq, ep)

		// if the endpoint is not in connected state, conn.RemoteAddr() will return nil
		// this protection may be not enough, but will help us debug the panic
		if conn.RemoteAddr() == nil {
			log.Warnln("TCP endpoint is not connected, current state: %v", tcp.EndpointState(ep.State()))
			conn.Close()
			return
		}

		target := getAddr(ep.Info().(*stack.TransportEndpointInfo).ID)
		tunnel.Add(adapters.NewSocket(target, conn, C.TUN))

	})
	ipstack.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpFwd.HandlePacket)

	// UDP handler
	ipstack.SetTransportProtocolHandler(udp.ProtocolNumber, tl.udpHandlePacket)

	log.Infoln("Tun adapter have interface name: %s", tundev.Name())
	return tl, nil

}

// Close close the TunAdapter
func (t *tunAdapter) Close() {
	t.device.Close()
	if t.dnsserver != nil {
		t.dnsserver.Stop()
	}
	t.ipstack.Close()
}

// IfName return device URL of tun
func (t *tunAdapter) DeviceURL() string {
	return t.device.URL()
}

func (t *tunAdapter) udpHandlePacket(id stack.TransportEndpointID, pkt *stack.PacketBuffer) bool {
	// ref: gvisor pkg/tcpip/transport/udp/endpoint.go HandlePacket
	hdr := header.UDP(pkt.TransportHeader().View())
	if int(hdr.Length()) > pkt.Data().Size()+header.UDPMinimumSize {
		// Malformed packet.
		t.ipstack.Stats().UDP.MalformedPacketsReceived.Increment()
		return true
	}

	target := getAddr(id)

	packet := &fakeConn{
		id:      id,
		pkt:     pkt,
		s:       t.ipstack,
		payload: pkt.Data().AsRange().ToOwnedView(),
	}
	tunnel.AddPacket(adapters.NewPacket(target, packet, C.TUN))

	return true
}

func getAddr(id stack.TransportEndpointID) socks5.Addr {
	ipv4 := id.LocalAddress.To4()

	// get the big-endian binary represent of port
	port := make([]byte, 2)
	binary.BigEndian.PutUint16(port, id.LocalPort)

	if ipv4 != "" {
		addr := make([]byte, 1+net.IPv4len+2)
		addr[0] = socks5.AtypIPv4
		copy(addr[1:1+net.IPv4len], []byte(ipv4))
		addr[1+net.IPv4len], addr[1+net.IPv4len+1] = port[0], port[1]
		return addr
	} else {
		addr := make([]byte, 1+net.IPv6len+2)
		addr[0] = socks5.AtypIPv6
		copy(addr[1:1+net.IPv6len], []byte(id.LocalAddress))
		addr[1+net.IPv6len], addr[1+net.IPv6len+1] = port[0], port[1]
		return addr
	}

}
