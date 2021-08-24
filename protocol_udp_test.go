package proxyproto

import (
	"bytes"
	"net"
	"testing"
)

func TestPassthroughUDP(t *testing.T) {
	packetConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Errorf("err: %v", err)
	}

	go func() {
		conn, err := net.Dial("udp", packetConn.LocalAddr().String())
		if err != nil {
			t.Errorf("err: %v", err)
		}
		defer conn.Close()

		conn.Write([]byte("ping"))
		recv := make([]byte, 4)
		_, err = conn.Read(recv)
		if err != nil {
			t.Errorf("err: %v", err)
		}
		if !bytes.Equal(recv, []byte("pong")) {
			t.Errorf("bad: %v", recv)
		}
	}()

	recv := make([]byte, 4)

	_, addr, err := packetConn.ReadFrom(recv)
	if err != nil {
		t.Errorf("err: %v", err)
	}

	if !bytes.Equal(recv, []byte("ping")) {
		t.Errorf("bad: %v", recv)
	}

	if _, err := packetConn.WriteTo([]byte("pong"), addr); err != nil {
		t.Errorf("err: %v", err)
	}
}

func TestUDPParse_ipv4(t *testing.T) {
	packetConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Errorf("err: %v", err)
	}

	packetConn = NewPacketConn(packetConn)

	header := &Header{
		Version:           2,
		Command:           PROXY,
		TransportProtocol: UDPv4,
		SourceAddr: &net.UDPAddr{
			IP:   net.ParseIP("10.1.1.1"),
			Port: 1000,
		},
		DestinationAddr: &net.UDPAddr{
			IP:   net.ParseIP("20.2.2.2"),
			Port: 2000,
		},
	}

	go func() {
		conn, err := net.Dial("udp", packetConn.LocalAddr().String())
		if err != nil {
			t.Errorf("err: %v", err)
		}
		defer conn.Close()

		// Write out the header!
		header.WriteTo(conn)

		conn.Write([]byte("ping"))
		recv := make([]byte, 4)
		_, err = conn.Read(recv)
		if err != nil {
			t.Errorf("err: %v", err)
		}
		if !bytes.Equal(recv, []byte("pong")) {
			t.Errorf("bad: %v", recv)
		}
	}()

	recv := make([]byte, 4)
	_, addr, err := packetConn.ReadFrom(recv)
	if err != nil {
		t.Errorf("err: %v", err)
	}
	if !bytes.Equal(recv, []byte("ping")) {
		t.Errorf("bad: %v", recv)
	}

	if _, err := packetConn.WriteTo([]byte("pong"), addr); err != nil {
		t.Errorf("err: %v", err)
	}

	// Check the remote addr
	if addr.String() != "10.1.1.1:1000" {
		t.Errorf("bad: %v", addr)
	}

	h := packetConn.(*PacketConn).ProxyHeader()
	if !h.EqualsTo(header) {
		t.Errorf("bad: %v", h)
	}
}

func TestUDPParse_ipv6(t *testing.T) {
	packetConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Errorf("err: %v", err)
	}

	packetConn = NewPacketConn(packetConn)

	header := &Header{
		Version:           2,
		Command:           PROXY,
		TransportProtocol: UDPv6,
		SourceAddr: &net.UDPAddr{
			IP:   net.ParseIP("ffff::ffff"),
			Port: 1000,
		},
		DestinationAddr: &net.UDPAddr{
			IP:   net.ParseIP("ffff::ffff"),
			Port: 2000,
		},
	}

	go func() {
		conn, err := net.Dial("udp", packetConn.LocalAddr().String())
		if err != nil {
			t.Errorf("err: %v", err)
		}
		defer conn.Close()

		// Write out the header!
		header.WriteTo(conn)

		conn.Write([]byte("ping"))
		recv := make([]byte, 4)
		_, err = conn.Read(recv)
		if err != nil {
			t.Errorf("err: %v", err)
		}
		if !bytes.Equal(recv, []byte("pong")) {
			t.Errorf("bad: %v", recv)
		}
	}()

	recv := make([]byte, 4)
	_, addr, err := packetConn.ReadFrom(recv)
	if err != nil {
		t.Errorf("err: %v", err)
	}
	if !bytes.Equal(recv, []byte("ping")) {
		t.Errorf("bad: %v", recv)
	}

	if _, err := packetConn.WriteTo([]byte("pong"), addr); err != nil {
		t.Errorf("err: %v", err)
	}

	// Check the remote addr
	if addr.String() != "[ffff::ffff]:1000" {
		t.Errorf("bad: %v", addr)
	}

	h := packetConn.(*PacketConn).ProxyHeader()
	if !h.EqualsTo(header) {
		t.Errorf("bad: %v", h)
	}
}
