package socks

import (
	"net"

	"github.com/Dreamacro/clash/common/pool"
	"github.com/Dreamacro/clash/component/socks5"
)

type fakeConn struct {
	net.PacketConn
	remoteAddr net.Addr
	targetAddr socks5.Addr
	payload    []byte
	bufRef     []byte
}

func (c *fakeConn) Data() []byte {
	return c.payload
}

// Wirte UDP packet with source(ip, port) = `addr`
func (c *fakeConn) WriteFrom(b []byte, addr net.Addr) (n int, err error) {
	var from socks5.Addr
	if addr == nil {
		// if addr is not provided, use the original source
		from = c.targetAddr
	} else {
		from = socks5.ParseAddrFromNetAddr(addr)
	}
	packet, err := socks5.EncodeUDPPacket(from, b)
	if err != nil {
		return
	}
	return c.PacketConn.WriteTo(packet, c.remoteAddr)
}

// LocalAddr returns the source IP/Port of UDP Packet
func (c *fakeConn) LocalAddr() net.Addr {
	return c.remoteAddr
}

func (c *fakeConn) Close() error {
	err := c.PacketConn.Close()
	pool.BufPool.Put(c.bufRef[:cap(c.bufRef)])
	return err
}
