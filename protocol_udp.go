package proxyproto

import (
	"bufio"
	"bytes"
	"net"
)

const readBufSize = 512

type ReadInfo struct {
	buf  []byte
	bufN int
	addr net.Addr
	err  error
}

// PacketConn provides a PROXY-aware wrapper around existing net.PacketConn
type PacketConn struct {
	net.PacketConn

	ProxyHeaderPolicy Policy
	Validate          Validator

	header   *Header
	readInfo *ReadInfo
	readErr  error
}

// NewPacketConn returns a new PacketConn
func NewPacketConn(conn net.PacketConn, opts ...func(*PacketConn)) *PacketConn {
	packetConn := &PacketConn{
		PacketConn: conn,
	}
	for _, opt := range opts {
		opt(packetConn)
	}
	return packetConn
}

// ReadFrom implements net.PacketConn
//
// It will parse PROXY header first, then copies the actual data into p.  On
// successful parse, the returned address will be of type *Addr
func (p *PacketConn) ReadFrom(b []byte) (n int, addr net.Addr, err error) {
	// Reset Header and ReadInfo everytime ReadFrom is called
	p.header = nil
	p.readInfo = nil
	//

	if p.readErr = p.readHeader(); p.readErr != nil {
		// Returns the address read by readHeader and the error
		return 0, p.readInfo.addr, p.readErr
	}

	if p.header != nil {
		n, addr, err = p.PacketConn.ReadFrom(b)

		// Overwrite addr
		addr = &Addr{
			remoteAddr: addr,
			Addr:       p.header.SourceAddr,
		}
	} else if p.readInfo.bufN > 0 {
		var j int = 0

		// Copy readBuf
		n = copy(b, p.readInfo.buf[:p.readInfo.bufN])

		// Continue reading the rest if necessary
		if p.readInfo.bufN > readBufSize {
			j, addr, err = p.PacketConn.ReadFrom(b[p.readInfo.bufN:])
		} else {
			addr = p.readInfo.addr
		}

		n = n + j

	} else {
		n, addr, err = p.PacketConn.ReadFrom(b)
	}

	return n, addr, err
}

// WriteTo ...
func (p *PacketConn) WriteTo(b []byte, addr net.Addr) (n int, err error) {
	switch addr := addr.(type) {
	case *Addr:
		return p.PacketConn.WriteTo(b, addr.RemoteAddr())
	}
	return p.PacketConn.WriteTo(b, addr)
}

// Raw returns the underlying connection which can be casted to
// a concrete type, allowing access to specialized functions.
//
// Use this ONLY if you know exactly what you are doing.
func (p *PacketConn) Raw() net.PacketConn {
	return p.PacketConn
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

// ProxyHeader returns the proxy protocol header, if any. If an error occurs
// while reading the proxy header, nil is returned.
func (p *PacketConn) ProxyHeader() *Header {
	if p.header == nil {
		p.readErr = p.readHeader()
	}
	return p.header
}

func (p *PacketConn) readHeader() error {
	rf := &ReadInfo{
		buf: make([]byte, readBufSize),
	}

	defer func() {
		p.readInfo = rf
	}()

	rf.bufN, rf.addr, rf.err = p.PacketConn.ReadFrom(rf.buf)
	if rf.err != nil {
		return nil
	}

	rb := bytes.NewReader(rf.buf[:rf.bufN])
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
