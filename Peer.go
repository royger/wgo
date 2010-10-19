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
	"sync"
	//"bytes"
	//"io"
	)

type Peer struct {
	addr, remote_peerId, our_peerId, infohash string
	numPieces int64
	wire *Wire
	bitfield *Bitfield
	our_bitfield *Bitfield
	in chan *message
	incoming chan *message // Exclusive channel, where peer receives messages and PeerMgr sends
	outgoing chan *string // Shared channel, peer sends messages and PeerMgr receives
	requests chan *PieceMgrRequest // Shared channel with the PieceMgr, used to request new pieces
	delete chan *message
	up_limit *time.Ticker
	down_limit *time.Ticker
	am_choking bool
	am_interested bool
	peer_choking bool
	peer_interested bool
	connected bool
	last bool
	received_keepalive int64
	writeQueue *PeerQueue
	mutex *sync.Mutex
	stats chan *Status
	//log *logger
	keepAlive *time.Ticker
	inFiles chan *FileMsg
	lastPiece int64
	is_incoming bool
}

func NewPeer(addr, infohash, peerId string, outgoing chan *string, numPieces int64, requests chan *PieceMgrRequest, our_bitfield *Bitfield, stats chan *Status, inFiles chan *FileMsg, up_limit *time.Ticker, down_limit *time.Ticker) (p *Peer, err os.Error) {
	p = new(Peer)
	p.mutex = new(sync.Mutex)
	p.addr = addr
	//p.log, err = NewLogger(p.addr)
	p.infohash = infohash
	p.our_peerId = peerId
	p.incoming = make(chan *message)
	p.in = make(chan *message)
	p.outgoing = outgoing
	p.inFiles = inFiles
	p.am_choking = true
	p.am_interested = false
	p.peer_choking = true
	p.peer_interested = false
	p.connected = false
	p.bitfield = NewBitfield(numPieces)
	p.our_bitfield = our_bitfield
	p.numPieces = numPieces
	p.requests = requests
	p.stats = stats
	p.delete = make(chan *message)
	// Start writting queue
	p.in = make(chan *message)
	p.keepAlive = time.NewTicker(KEEP_ALIVE_MSG)
	p.writeQueue = NewQueue(p.incoming, p.in, p.delete)
	p.up_limit = up_limit
	p.down_limit = down_limit
	p.lastPiece = time.Seconds()
	go p.writeQueue.Run()
	return
}

func NewPeerFromConn(conn net.Conn, infohash, peerId string, outgoing chan *string, numPieces int64, requests chan *PieceMgrRequest, our_bitfield *Bitfield, stats chan *Status, inFiles chan *FileMsg, up_limit *time.Ticker, down_limit *time.Ticker) (p *Peer, err os.Error) {
	addr := conn.RemoteAddr().String()
	p, err = NewPeer(addr, infohash, peerId, outgoing, numPieces, requests, our_bitfield, stats, inFiles, up_limit, down_limit)
	p.wire, err = NewWire(p.infohash, p.our_peerId, conn, p.up_limit, p.down_limit, p.inFiles)
	p.is_incoming = true
	return
}

func (p *Peer) preprocessMessage(msg *message) (skip bool, err os.Error) {
	if msg == nil {
		err = os.NewError("Nil message")
		return
	}
	switch msg.msgId {
		case unchoke:
			if !p.am_choking {
				// Avoid sending repeated unchoke messages
				skip = true
			} else {
				p.am_choking = false
			}
		case choke:
			if p.am_choking {
				// Avoid sending repeated choke messages
				skip = true
			} else {
				p.am_choking = true
				// Flush peer request queue
				p.incoming <- &message{length: 1, msgId: flush}
			}
		case interested:
			if p.am_interested {
				skip = true
			} else {
				p.am_interested = true
			}
		case uninterested:
			if !p.am_interested {
				skip = true
			} else {
				p.am_interested = false
			}
		case have:
			p.CheckInterested()
		case piece:
			msg.length = uint32(9 + int64(binary.BigEndian.Uint32(msg.payLoad[8:12])))
			msg.payLoad = msg.payLoad[0:8]
	}
	return
}


func (p *Peer) PeerWriter() {
	// Create connection
	defer p.Close()
	var err os.Error
	if p.wire == nil {
		addrTCP, err := net.ResolveTCPAddr(p.addr)
		if err != nil {
			//p.log.Output(err, p.addr)
			return
		}
		conn, err := net.DialTCP("tcp4", nil, addrTCP)
		if err != nil {
			//p.log.Output(err, p.addr)
			return
		}
		/*err = conn.SetTimeout(TIMEOUT)
		if err != nil {
			//p.log.Output(err, p.addr)
			return
		}*/
		// Create the wire struct
		p.wire, err = NewWire(p.infohash, p.our_peerId, conn, p.up_limit, p.down_limit, p.inFiles)
		if err != nil {
			return
		}
	}
	// Send handshake
	p.remote_peerId, err = p.wire.Handshake()
	if err != nil {
		//p.log.Output(err, p.is_incoming, p.addr)
		return
	}
	if p.remote_peerId == p.our_peerId {
		//p.log.Output("Local loopback")
		return
	}
	// Launch peer reader
	go p.PeerReader()
	// Send the have message
	our_bitfield := p.our_bitfield.Bytes()
	err = p.wire.WriteMsg(&message{length: uint32(1 + len(our_bitfield)), msgId: bitfield, payLoad: our_bitfield})
	if err != nil {
		//p.log.Output(err, p.is_incoming, p.addr)
		return
	}
	// Peer writer main bucle
	p.connected = true
	for !closed(p.in) {
		//p.log.Output("PeerWriter -> Waiting for message to send to", p.addr)
		select {
			// Wait for messages or send keep-alive
			case msg := <- p.in:
				skip, err := p.preprocessMessage(msg)
				if err != nil {
					log.Println("Peer -> Error:", err)
					return
				}
				if skip {
					continue
				}
				err = p.wire.WriteMsg(msg)
				if err != nil /*|| n != int(4+msg.length)*/ {
					//p.log.Output(err, p.addr, err)
					return
				}
				// Send message to StatMgr
				if msg.msgId == piece {
					statMsg := new(Status)
					statMsg.downloaded = int64(msg.length - 9)
					statMsg.addr = p.addr
					p.stats <- statMsg
					//log.Println(statMsg)
				}
				// Reset ticker
				//close(p.keepAlive)
				p.keepAlive.Stop()
				p.keepAlive = time.NewTicker(KEEP_ALIVE_MSG)
				//p.log.Output("PeerWriter -> Finished sending message with id:", msg.msgId, "to", p.addr)
			case <- p.keepAlive.C:
				// Send keep-alive
				//p.log.Output("PeerWriter -> Sending Keep-Alive message to", p.addr)
				err := p.wire.WriteMsg(&message{length: 0})
				if err != nil {
					//p.log.Output(err, p.addr, err)
					return
				}
				//p.log.Output("PeerWriter -> Finished sending Keep-Alive message to", p.addr)
		}
	}
}

func (p *Peer) PeerReader() {
	defer p.Close()
	piece_buf := make([]byte, STANDARD_BLOCK_LENGTH)
	for p.wire != nil {
		//p.log.Output("PeerReader -> Waiting for message from peer", p.addr)
		msg, err := p.wire.ReadMsg(piece_buf)
		if err != nil {
			//p.log.Output(err, p.addr)
			return
		}
		//p.log.Output("PeerReader -> Received message from", p.addr)
		if msg.length == 0 {
			p.received_keepalive = time.Seconds()
		} else {
			if msg.msgId == piece {
				statMsg := new(Status)
				statMsg.uploaded = int64(msg.length - 9)
				statMsg.addr = p.addr
				p.stats <- statMsg
			}
			err := p.ProcessMessage(msg)
			if err != nil {
				//p.log.Output(err, p.addr)
				return
			}
		}
		//p.log.Output("PeerReader -> Finished processing message fromr", p.addr)
	}
}

func (p *Peer) ProcessMessage(msg *message) (err os.Error){
	//p.log.Output("Processing message with id:", msg.msgId)
	switch msg.msgId {
		case choke:
			// Choke peer
			p.peer_choking = true
			//p.log.Output("Peer", p.addr, "choked")
			// If choked, clear request list
			//p.log.Output("Cleaning request list")
			p.requests <- &PieceMgrRequest{msg: &message{length: 1, msgId: exit, addr: []string{p.addr}}}
			//p.log.Output("Finished cleaning")
		case unchoke:
			// Unchoke peer
			p.peer_choking = false
			//log.Println("Peer", p.addr, "unchoked")
			// Check if we are still interested on this peer
			//p.CheckInterested()
			// Notice PieceMgr of the unchoke
			p.TryToRequestPiece()
		case interested:
			// Mark peer as interested
			p.peer_interested = true
			//log.Println("Peer", p.addr, "interested")
		case uninterested:
			// Mark peer as uninterested
			p.peer_interested = false
			//log.Println("Peer", p.addr, "uninterested")
		case have:
			// Update peer bitfield
			p.bitfield.Set(int64(binary.BigEndian.Uint32(msg.payLoad)))
			if p.our_bitfield.Completed() && p.bitfield.Completed() {
				err = os.NewError("Peer not useful")
				return
			}
			p.CheckInterested()
			//log.Println("Peer", p.addr, "have")
			// If we are unchoked notice PieceMgr of the new piece
			p.TryToRequestPiece()
		case bitfield:
			// Set peer bitfield
			//log.Println(msg)
			p.bitfield, err = NewBitfieldFromBytes(p.numPieces, msg.payLoad)
			if err != nil {
				return os.NewError("Invalid bitfield")
			}
			if p.our_bitfield.Completed() && p.bitfield.Completed() {
				err = os.NewError("Peer not useful")
				return
			}
			p.CheckInterested()
			p.TryToRequestPiece()
			//log.Println("Peer", p.addr, "bitfield")
		case request:
			// Peer requests a block
			//log.Println("Peer", p.addr, "requests a block")
			if !p.am_choking {
				p.requests <- &PieceMgrRequest{msg: msg, response: p.incoming}
			}
			//log.Println("Peer -> Received request from", p.addr)
		case piece:
			//p.log.Output("Received piece, sending to pieceMgr")
			p.requests <- &PieceMgrRequest{msg: msg}
			p.lastPiece = time.Seconds()
			// Check if the peer is still interesting
			//p.log.Output("Checking if interesting")
			// p.CheckInterested()
			// Try to request another block
			//p.log.Output("Trying to request a new piece")
			p.TryToRequestPiece()
			//p.log.Output("Finished requesting new piece")
		case cancel:
			// Send the message to the sending queue to delete the "piece" message
			p.delete <- msg
		case port:
			// DHT stuff
		default:
			//p.log.Output("Unknown message")
			return os.NewError("Unknown message")
	}
	//p.log.Output("Finished processing")
	return
}

func (p *Peer) CheckInterested() {
	if p.am_interested && p.our_bitfield.Completed() {
		p.incoming <- &message{length: 1, msgId: uninterested}
		return
	}
	bitfield := p.bitfield.Bytes()
	if p.am_interested && !p.our_bitfield.HasMorePieces(bitfield) {
		//p.am_interested = false
		p.incoming <- &message{length: 1, msgId: uninterested}
		//log.Println("Peer", p.addr, "marked as uninteresting")
		return
	}
	if !p.am_interested && p.our_bitfield.HasMorePieces(bitfield) {
		//p.am_interested = true
		p.incoming <- &message{length: 1, msgId: interested}
		//log.Println("Peer", p.addr, "marked as interesting")
		return
	}
}

func (p *Peer) TryToRequestPiece() {
	if !p.peer_choking && !p.our_bitfield.Completed() {
		//p.log.Output("Sending request for new piece")
		p.requests <- &PieceMgrRequest{bitfield: p.bitfield, response: p.incoming, our_addr: p.addr, msg: &message{length: 1, msgId: our_request}}
		//p.log.Output("Finished sending request for new piece")
	}
}

func (p *Peer) Close() {
	//p.log.Output("Finishing peer")
	p.mutex.Lock()
	defer p.mutex.Unlock()
	//p.log.Output("Sending message to peerMgr")
	p.outgoing <- &p.addr
	//p.log.Output("Finished sending message")
	//p.log.Output("Sending message to pieceMgr")
	p.requests <- &PieceMgrRequest{msg: &message{length: 1, msgId: exit, addr: []string{p.addr}}}
	//p.log.Output("Finished sending message")
	// Sending message to Stats
	p.stats <- &Status{uploaded: 0, downloaded: 0, addr: p.addr}
	// Finished
	close(p.incoming)
	close(p.in)
	close(p.delete)
	p.keepAlive.Stop()
	// Here we could have a crash
	if p.wire != nil {
		p.wire.Close()
	}
	if p.last {
		p.wire = nil
		p.bitfield = nil
		p.our_bitfield = nil
		p.writeQueue = nil
		p.keepAlive = nil
		//p.log.Output("Removed all info")
	} else {
		p.last = true
	} 
	//p.log.Output("Finished closing peer")
}
