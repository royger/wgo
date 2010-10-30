// Handles a peer
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package peers

import(
	"log"
	"os"
	"net"
	"time"
	"encoding/binary"
	"sync"
	"wgo/limiter"
	"wgo/bit_field"
	"wgo/files"
	"wgo/stats"
	)
	
const(
	KEEP_ALIVE_MSG = 120*NS_PER_S
)

type Peer struct {
	addr, remote_peerId, our_peerId, infohash string
	numPieces int64
	wire *Wire
	bitfield *bit_field.Bitfield
	our_bitfield *bit_field.Bitfield
	in chan *message
	incoming chan *message // Exclusive channel, where peer receives messages and PeerMgr sends
	//outgoing chan *string // Shared channel, peer sends messages and PeerMgr receives
	peerMgr PeerMgr
	//requests chan *PieceMgrRequest // Shared channel with the PieceMgr, used to request new pieces
	pieceMgr PieceMgr
	delete chan *message
	//up_limit *time.Ticker
	//down_limit *time.Ticker
	l limiter.Limiter
	am_choking bool
	am_interested bool
	peer_choking bool
	peer_interested bool
	connected bool
	last bool
	received_keepalive int64
	writeQueue *PeerQueue
	mutex *sync.Mutex
	stats stats.Stats
	//log *logger
	keepAlive *time.Ticker
	//inFiles chan *FileMsg
	files files.Files
	lastPiece int64
	is_incoming bool
}

func (p *Peer) Choke() {
	p.incoming <- &message{length: 1, msgId: choke}
}

func (p *Peer) Unchoke() {
	p.incoming <- &message{length: 1, msgId: unchoke}
}

func (p *Peer) Connected() bool {
	return p.connected
}

func (p *Peer) Completed() bool {
	return p.bitfield.Completed()
}

func (p *Peer) Am_choking() bool {
	return p.am_choking
}

func (p *Peer) Peer_choking() bool {
	return p.peer_choking
}

func (p *Peer) Am_interested() bool {
	return p.am_interested
}

func (p *Peer) Peer_interested() bool {
	return p.peer_interested
}

func (p *Peer) LastPiece() int64 {
	return p.lastPiece
}

func (p *Peer) Request(piece int64, block int) {
	msg := new(message)
	begin := int64(block) * int64(STANDARD_BLOCK_LENGTH)
	length := int64(STANDARD_BLOCK_LENGTH)
	if piece == p.numPieces-1 {
		left := p.lastPiece - begin
		if left < length {
			length = left
		}
	}
	//log.Println("Requesting", piece, ".", block)
	msg.msgId = request
	msg.payLoad = make([]byte, 12)
	msg.length = uint32(1 + len(msg.payLoad))
	binary.BigEndian.PutUint32(msg.payLoad[0:4], uint32(piece))
	binary.BigEndian.PutUint32(msg.payLoad[4:8], uint32(begin))
	binary.BigEndian.PutUint32(msg.payLoad[8:12], uint32(length))
	p.incoming <- msg
}

func NewPeer(addr, infohash, peerId string, peerMgr PeerMgr, numPieces int64, pieceMgr PieceMgr, our_bitfield *bit_field.Bitfield, st stats.Stats, fl files.Files, l limiter.Limiter) (p *Peer, err os.Error) {
	p = new(Peer)
	p.mutex = new(sync.Mutex)
	p.addr = addr
	//p.log, err = NewLogger(p.addr)
	p.infohash = infohash
	p.our_peerId = peerId
	p.incoming = make(chan *message)
	//p.in = make(chan *message)
	//p.outgoing = outgoing
	//p.inFiles = inFiles
	p.files = fl
	p.am_choking = true
	p.am_interested = false
	p.peer_choking = true
	p.peer_interested = false
	p.connected = false
	p.bitfield = bit_field.NewBitfield(numPieces)
	p.our_bitfield = our_bitfield
	p.numPieces = numPieces
	//p.requests = requests
	p.pieceMgr = pieceMgr
	p.peerMgr = peerMgr
	p.stats = st
	p.delete = make(chan *message)
	// Start writting queue
	p.in = make(chan *message)
	p.keepAlive = time.NewTicker(KEEP_ALIVE_MSG)
	p.writeQueue = NewQueue(p.incoming, p.in, p.delete)
	//p.up_limit = up_limit
	//p.down_limit = down_limit
	p.l = l
	p.lastPiece = time.Seconds()
	go p.writeQueue.Run()
	return
}

func NewPeerFromConn(conn net.Conn, infohash, peerId string, peerMgr PeerMgr, numPieces int64, pieceMgr PieceMgr, our_bitfield *bit_field.Bitfield, st stats.Stats, fl files.Files, l limiter.Limiter) (p *Peer, err os.Error) {
	addr := conn.RemoteAddr().String()
	p, err = NewPeer(addr, infohash, peerId, peerMgr, numPieces, pieceMgr, our_bitfield, st, fl, l)
	p.wire, err = NewWire(p.infohash, p.our_peerId, conn, p.l, fl)
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
		p.wire, err = NewWire(p.infohash, p.our_peerId, conn, p.l, p.files)
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
					p.stats.Update(p.addr, 0, int64(msg.length - 9))
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
			log.Println("Peer -> Reader:", err)
			return
		}
		//p.log.Output("PeerReader -> Received message from", p.addr)
		if msg.length == 0 {
			p.received_keepalive = time.Seconds()
		} else {
			if msg.msgId == piece {
				p.stats.Update(p.addr, int64(msg.length - 9), 0)
			}
			err := p.ProcessMessage(msg)
			if err != nil {
				log.Println("Peer -> Reader:", err)
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
			p.pieceMgr.PeerExit(p.addr)
			//p.requests <- &PieceMgrRequest{msg: &message{length: 1, msgId: exit, addr: []string{p.addr}}}
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
			p.bitfield, err = bit_field.NewBitfieldFromBytes(p.numPieces, msg.payLoad)
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
				// p.requests <- &PieceMgrRequest{msg: msg, response: p.incoming}
				if msg.length < 9 {
					return os.NewError("Unexpected message length")
				}
				index := binary.BigEndian.Uint32(msg.payLoad[0:4])
				if !p.our_bitfield.IsSet(int64(index)) {
					return os.NewError("Peer requests unfinished piece, ignoring request")
				}
				msg.msgId = piece
				p.incoming <- msg
			}
			//log.Println("Peer -> Received request from", p.addr)
		case piece:
			//p.log.Output("Received piece, sending to pieceMgr")
			//p.requests <- &PieceMgrRequest{msg: msg}
			p.pieceMgr.SavePiece(p.addr, int64(binary.BigEndian.Uint32(msg.payLoad[0:4])), int64(binary.BigEndian.Uint32(msg.payLoad[4:8])), int64(msg.length-9))
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
	bf := p.bitfield.Bytes()
	if p.am_interested && !p.our_bitfield.HasMorePieces(bf) {
		//p.am_interested = false
		p.incoming <- &message{length: 1, msgId: uninterested}
		//log.Println("Peer", p.addr, "marked as uninteresting")
		return
	}
	if !p.am_interested && p.our_bitfield.HasMorePieces(bf) {
		//p.am_interested = true
		p.incoming <- &message{length: 1, msgId: interested}
		//log.Println("Peer", p.addr, "marked as interesting")
		return
	}
}

func (p *Peer) TryToRequestPiece() {
	if !p.peer_choking && !p.our_bitfield.Completed() {
		//p.log.Output("Sending request for new piece")
		p.pieceMgr.Request(p.addr, p, p.bitfield)
		//p.requests <- &PieceMgrRequest{bitfield: p.bitfield, response: p.incoming, our_addr: p.addr, msg: &message{length: 1, msgId: our_request}}
		//p.log.Output("Finished sending request for new piece")
	}
}

func (p *Peer) Close() {
	//p.log.Output("Finishing peer")
	p.mutex.Lock()
	defer p.mutex.Unlock()
	//p.log.Output("Sending message to peerMgr")
	p.peerMgr.DeletePeer(p.addr)
	//p.outgoing <- &p.addr
	//p.log.Output("Finished sending message")
	//p.log.Output("Sending message to pieceMgr")
	//p.requests <- &PieceMgrRequest{msg: &message{length: 1, msgId: exit, addr: []string{p.addr}}}
	p.pieceMgr.PeerExit(p.addr)
	//p.log.Output("Finished sending message")
	// Sending message to Stats
	p.stats.Update(p.addr, 0, 0)
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
