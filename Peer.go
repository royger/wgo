// Handles a peer
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package main

import(
	"log"
	"os"
	"net"
	"time"
	"encoding/binary"
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
	err = conn.SetTimeout(TIMEOUT)
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
	// Launch peer reader
	go p.PeerReader()
	// keep alive ticker
	keepAlive := time.Tick(KEEP_ALIVE_MSG)
	// Peer writer main bucle
	for {
		select {
			// Wait for messages or send keep-alive
			case msg := <- p.incoming:
				// New message to send
				err := p.wire.WriteMsg(msg)
				if err != nil {
					p.outgoing <- message{length: 1, msgId: exit, addr: p.addr}
					return
				}
				// Reset ticker
				keepAlive = time.Tick(KEEP_ALIVE_MSG)
			case <- keepAlive:
				// Send keep-alive
				log.Stderr("Sending Keep-Alive message", p.addr)
				err := p.wire.WriteMsg(message{length: 0})
				if err != nil {
					p.outgoing <- message{length: 1, msgId: exit, addr: p.addr}
					return
				}
		}
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
		if msg.length == 0 {
			log.Stderr("Received keep-alive from", p.addr)
			p.received_keepalive = time.Seconds()
		} else {
			err := p.ProcessMessage(*msg)
			if err != nil {
				p.outgoing <- message{length: 1, msgId: exit, addr: p.addr}
				return
			}
		}
	}
}

func (p *Peer) ProcessMessage(msg message) (err os.Error){
	switch msg.msgId {
		case choke:
			// Choke peer
			p.peer_choking = true
			log.Stderr("Peer", p.addr, "choked")
		case unchoke:
			// Unchoke peer
			p.peer_choking = false
			log.Stderr("Peer", p.addr, "unchoked")
		case interested:
			// Mark peer as interested
			p.peer_interested = true
			log.Stderr("Peer", p.addr, "interested")
		case uninterested:
			// Mark peer as uninterested
			p.peer_interested = false
			log.Stderr("Peer", p.addr, "uninterested")
		case have:
			// Update peer bitfield
			p.bitfield.Set(int(binary.BigEndian.Uint32(msg.payLoad)))
			log.Stderr("Peer", p.addr, "have")
		case bitfield:
			// Set peer bitfield
			p.bitfield, err = NewBitfieldFromBytes(int(p.numPieces), msg.payLoad)
			if err != nil {
				return os.NewError("Invalid bitfield")
			}
			log.Stderr("Peer", p.addr, "bitfield")
		case request:
			// Peer requests a block
		case piece:
			// We have received a piece
		case cancel:
			// Cancel a previous request
		case port:
			// DHT stuff
		default:
			return os.NewError("Unknown message")
	}
	return
}

func (p *Peer) Close() {
	if p.wire != nil {
		p.wire.Close()
		// Send dummy incoming message to flush PeerWriter
		_ = p.incoming <- message{length: 0, addr: p.addr}
	}
}
