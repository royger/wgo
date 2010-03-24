// Handles a peer
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package main

import(
	"log"
	"os"
	"net"
	"time"
	)

type Peer struct {
	addr, remote_peerId, our_peerId, infohash string
	numPieces int64
	wire *Wire
	bitfield *Bitfield
	incoming chan message // Exclusive channel, where peer receives messages and PeerMgr sends
	outgoing chan message // Shared channel, peer sends messages and PeerMgr receives
	am_choking bool
	am_interested bool
	peer_choking bool
	peer_interested bool
	received_keepalive int64
	last_activity int64
}

func NewPeer(addr, infohash, peerId string, incoming, outgoing chan message, numPieces int64) (p *Peer, err os.Error) {
	p = new(Peer)
	p.addr = addr
	p.infohash = infohash
	p.our_peerId = peerId
	p.incoming = incoming
	p.outgoing = outgoing
	p.am_choking = true
	p.am_interested = false
	p.peer_choking = true
	p.peer_interested = false
	p.bitfield = NewBitfield(int(numPieces))
	p.numPieces = numPieces
	return
}

func (p *Peer) PeerWriter() {
	// Create connection
	addrTCP, err := net.ResolveTCPAddr(p.addr)
	if err != nil {
		p.outgoing <- message{length: 1, msgId: exit, addr: p.addr}
		return
	}
	log.Stderr("Connecting to", p.addr)
	conn, err := net.DialTCP("tcp4", nil, addrTCP)
	if err != nil {
		p.outgoing <- message{length: 1, msgId: exit, addr: p.addr}
		return
	}
	defer p.Close()
	err = conn.SetTimeout(int64(TIMEOUT))
	if err != nil {
		p.outgoing <- message{length: 1, msgId: exit, addr: p.addr}
		return
	}
	// Create the wire struct
	p.wire = NewWire(p.infohash, p.our_peerId, conn)
	log.Stderr("Sending Handshake")
	// Send handshake
	p.remote_peerId, err = p.wire.Handshake()
	if err != nil {
		p.outgoing <- message{length: 1, msgId: exit, addr: p.addr}
	}
	p.last_activity = time.Seconds()
	// Launch peer reader
	go p.PeerReader()
	// Peer writer main bucle
	for msg := range p.incoming {
		err := p.wire.WriteMsg(msg)
		if err != nil {
			p.outgoing <- message{length: 1, msgId: exit, addr: p.addr}
			return
		}
		p.last_activity = time.Seconds()
	}
}

func (p *Peer) PeerReader() {
	defer p.Close()
	for p.wire != nil {
		msg, err := p.wire.ReadMsg()
		if err != nil {
			p.outgoing <- message{length: 1, msgId: exit, addr: p.addr}
			return
		}
		p.outgoing <- *msg
	}
}

func (p *Peer) Close() {
	if p.wire != nil {
		p.wire.Close()
		// Send dummy incoming message to flush PeerWriter
		_ = p.incoming <- message{length: 0, addr: p.addr}
	}
}
