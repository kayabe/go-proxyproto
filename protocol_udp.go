package proxyproto

import (
	"bufio"
	"bytes"
	"net"
	"sync"
)

// PacketConn provides a PROXY-aware wrapper around existing net.PacketConn
type PacketConn struct {
	net.PacketConn

	header            *Header
	once              sync.Once
	ProxyHeaderPolicy Policy
	Validate          Validator
	readErr           error
}

// NewPacketConn returns a new PacketConn
func NewPacketConn(conn net.PacketConn) *PacketConn {
	packetConn := &PacketConn{
		PacketConn: conn,
	}
	return packetConn
}

// ReadFrom implements net.PacketConn
//
// It will parse PROXY header first, then copies the actual data into p.  On
// successful parse, the returned address will be of type *Addr
func (p *PacketConn) ReadFrom(b []byte) (int, net.Addr, error) {
	p.readErr = p.readHeader()
	if p.readErr != nil {
		return 0, nil, p.readErr
	}

	n, orig, origErr := p.PacketConn.ReadFrom(b)

	if p.header != nil {

		return n, &Addr{
			Addr:       p.header.SourceAddr,
			remoteAddr: orig,
		}, origErr
	}

	return n, orig, origErr
}

// WriteTo ...
func (p *PacketConn) WriteTo(b []byte, addr net.Addr) (n int, err error) {
	return p.PacketConn.WriteTo(b, addr.(*Addr).RemoteAddr())
}

// LocalAddr returns the address of the server if the proxy
// protocol is being used, otherwise just returns the address of
// the socket server. In case an error happens on reading the
// proxy header the original LocalAddr is returned, not the one
// from the proxy header even if the proxy header itself is
// syntactically correct.
func (p *PacketConn) LocalAddr() net.Addr {
	if p.header == nil || p.header.Command.IsLocal() || p.readErr != nil {
		return p.PacketConn.LocalAddr()
	}

	return p.header.DestinationAddr
}

// Raw returns the underlying connection which can be casted to
// a concrete type, allowing access to specialized functions.
//
// Use this ONLY if you know exactly what you are doing.
func (p *PacketConn) Raw() net.PacketConn {
	return p.PacketConn
}

// ProxyHeader returns the proxy protocol header, if any. If an error occurs
// while reading the proxy header, nil is returned.
func (p *PacketConn) ProxyHeader() *Header {
	return p.header
}

func (p *PacketConn) readHeader() error {
	tmpBuf := make([]byte, 512)

	n, _, _ := p.PacketConn.ReadFrom(tmpBuf)
	if n <= 0 {
		return nil
	}

	rb := bytes.NewReader(tmpBuf[:n])
	br := bufio.NewReader(rb)

	header, err := Read(br)
	// For the purpose of this wrapper shamefully stolen from armon/go-proxyproto
	// let's act as if there was no error when PROXY protocol is not present.
	if err == ErrNoProxyProtocol {
		// but not if it is required that the connection has one
		if p.ProxyHeaderPolicy == REQUIRE {
			return err
		}

		return nil
	}

	// proxy protocol header was found
	if err == nil && header != nil {
		switch p.ProxyHeaderPolicy {
		case REJECT:
			// this connection is not allowed to send one
			return ErrSuperfluousProxyHeader
		case USE, REQUIRE:
			if p.Validate != nil {
				err = p.Validate(header)
				if err != nil {
					return err
				}
			}

			p.header = header
		}
	}

	return err
}

// Addr provides a way for PacketConn.ReadFrom to return to its caller both the
// endpoint address and addresses in the PROXY header
type Addr struct {
	net.Addr
	remoteAddr net.Addr
}

// RemoteAddr returns remote address in PROXY header
func (pa *Addr) RemoteAddr() net.Addr {
	return pa.remoteAddr
}
