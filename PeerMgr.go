// Management of a set of peers for a single torrent.
// Comunicates with the peers (readers/writers)
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package main

import(
	"os"
	"log"
	"container/list"
	"time"
	"net"
	"strings"
	)

// We will use 1 channel to send the data from all peers (Readers)
// We will use separate channels to comunicate with the peers (Writers)
// So this implementation will use 1+num_peers channels


// General structure that will hold comunitacion channels
// and info about peers

type PeerMgr struct {
	incoming chan *message // Shared channel where peer sends messages and PeerMgr receives
	peerMgr chan *message
	inListener chan net.Conn
	activePeers map[string] *Peer // List of active peers
	incomingPeers map[string] *Peer // List of incoming connections
	unusedPeers *list.List
	tracker <- chan peersList // Channel used to comunicate the Tracker thread and the PeerMgr
	inTracker chan <- int
	requests chan *PieceMgrRequest
	stats chan *Status
	inChokeMgr chan chan map[string]*Peer
	our_bitfield *Bitfield
	numPieces int64
	infohash, peerid string
	inFiles chan *FileMsg
	up_limit *time.Ticker
	down_limit *time.Ticker
}

// Create a PeerMgr

func NewPeerMgr(tracker chan peersList, inTracker chan int, numPieces int64, peerid, infohash string, requests chan *PieceMgrRequest, peerMgr chan *message, our_bitfield *Bitfield, stats chan *Status, inListener chan net.Conn, inChokeMgr chan chan map[string]*Peer, inFiles chan *FileMsg, up_limit *time.Ticker, down_limit *time.Ticker) (p *PeerMgr, err os.Error) {
	p = new(PeerMgr)
	p.incoming = make(chan *message)
	p.tracker = tracker
	p.inTracker = inTracker
	p.numPieces = numPieces
	p.infohash = infohash
	p.peerid = peerid
	p.activePeers = make(map[string] *Peer, ACTIVE_PEERS)
	p.incomingPeers = make(map[string] *Peer, INCOMING_PEERS)
	p.unusedPeers = list.New()
	p.requests = requests
	p.our_bitfield = our_bitfield
	p.peerMgr = peerMgr
	p.stats = stats
	p.inListener = inListener
	p.inChokeMgr = inChokeMgr
	p.inFiles = inFiles
	p.up_limit = up_limit
	p.down_limit = down_limit
	return
}

// Process messages from peers and do actions

func (p *PeerMgr) Run() {
	/*chokeRound := time.Tick(10*NS_PER_S)*/
	for {
		//log.Println("PeerMgr -> Waiting for messages")
		select {
			case msg := <- p.incoming:
				//log.Println("PeerMgr -> Processing Peer message")
				//log.Println(msg)
				/*err :=*/ p.ProcessPeerMessage(msg)
				/*if err != nil {
					log.Println(err)
				}*/
				//log.Println("PeerMgr -> Finished processing Peer message")
			case msg := <- p.tracker:
				//log.Println("PeerMgr -> Processing Tracker list")
				p.ProcessTrackerMessage(msg)
				//log.Println("PeerMgr -> Finished processing Tracker list. Active peers:", len(p.activePeers), "Inactive peers:", len(p.inactivePeers))
			case msg := <- p.peerMgr:
				//log.Println("PeerMgr -> Broadcasting message")
				// Broadcast have message
				p.Broadcast(msg)
				//log.Println("PeerMgr -> Finished broadcasting peer message")
			case c := <- p.inListener:
				// Incoming connection
				p.AddIncomingPeer(c)
			case c := <- p.inChokeMgr:
				peers := make(map[string]*Peer)
				for addr, peer := range(p.activePeers) {
					peers[addr] = peer
				}
				for addr, peer := range(p.incomingPeers) {
					peers[addr] = peer
				}
				c <- peers
				<- p.inChokeMgr
		}
	}
}

func (p *PeerMgr) ProcessPeerMessage(msg *message) (err os.Error) {
	//log.Println("Searching peer...")
	peer, err := p.SearchPeer(msg.addr[0])
	//log.Println("Searching peer finished")
	if err != nil {
		//log.Println("Peer not found")
		return
	}
	//log.Println("Message:", msg)
	switch msg.msgId {
		case exit:
			// Internal message used to remove a peer
			//log.Println("PeerMgr -> Removing peer", peer.addr)
			p.Remove(peer)
			//log.Println("Peer", peer.addr, "removed")
		default:
			log.Println("PeerMgr -> Unknown message ID")
	}
	return
}

// Broadcast message to all peers

func (p *PeerMgr) Broadcast(msg *message) {
	if len(msg.addr) == 0 {
		// Broadcast message to all peers
		//log.Println("Broadcasting have message")
		for _, peer := range(p.activePeers) {
			peer.incoming <- msg
		}
		for _, peer := range(p.incomingPeers) {
			peer.incoming <- msg
		}
		//log.Println("Finished broadcasting have message")
	} else {
		// Send message to some peers only
		for _, addr := range(msg.addr) {
			if peer, ok := p.activePeers[addr]; ok {
				peer.incoming <- msg
			} else if peer, ok := p.incomingPeers[addr]; ok {
				peer.incoming <- msg
			}
		}
	}
}

// Tracker sends us new peers

func (p *PeerMgr) ProcessTrackerMessage(msg peersList) {
	// See if activePeers list is not full
	//var err os.Error
	for i, addr := len(p.activePeers), msg.peers.Front(); i < ACTIVE_PEERS && addr != nil; i, addr = i+1, msg.peers.Front() {
		//log.Println("PeerMgr -> Adding Active Peer:", addr.Value.(string))
		if _, err := p.SearchPeer(addr.Value.(string)); err != nil {
			p.activePeers[addr.Value.(string)], err = NewPeer(addr.Value.(string), p.infohash, p.peerid, p.incoming, p.numPieces, p.requests, p.our_bitfield, p.stats, p.inFiles, p.up_limit, p.down_limit)
			if err != nil {
				log.Println("PeerMgr -> Error creating peer:", err)
			}
			go p.activePeers[addr.Value.(string)].PeerWriter()
		}
		msg.peers.Remove(addr)
	}
	p.unusedPeers.PushBackList(msg.peers)
}

// Search the peer

func (p *PeerMgr) SearchPeer(addr string) (peer *Peer, err os.Error) {
	var ok bool
	if peer, ok = p.activePeers[addr]; ok {
		return
	}
	/*if peer, ok = p.inactivePeers[addr]; ok {
		return
	}*/
	if peer, ok = p.incomingPeers[addr]; ok {
		return
	}
	return peer, os.NewError("PeerMgr -> Peer " + addr + " not found")
}

// Remove a peer

func (p *PeerMgr) Remove(peer *Peer) {
	//peer.Close()
	if _, ok := p.activePeers[peer.addr]; ok {
		p.activePeers[peer.addr] = peer, false
		p.AddNewPeer()
		return
	}
	if _, ok := p.incomingPeers[peer.addr]; ok {
		p.incomingPeers[peer.addr] = peer, false
		return
	}
}

// Add a new peer to the activePeers map

func (p *PeerMgr) AddNewPeer() (err os.Error) {
	addr := p.unusedPeers.Front()
	if addr == nil {
		// Requests new peers to the tracker module (check inactive peers & active peers also)
		p.inTracker <- (UNUSED_PEERS + (ACTIVE_PEERS - len(p.activePeers)))
		return os.NewError("Unused peers list is empty")
	}
	// Check how much of the unsued peers list is used, and request more if needed
	if (p.unusedPeers.Len()/UNUSED_PEERS * 100) < PERCENT_UNUSED_PEERS {
		// request new peers to tracker
		p.inTracker <- (UNUSED_PEERS - p.unusedPeers.Len())
	}
	//log.Println("Adding Inactive Peer:", addr.Value.(string))
	p.activePeers[addr.Value.(string)], _ = NewPeer(addr.Value.(string), p.infohash, p.peerid, p.incoming, p.numPieces, p.requests, p.our_bitfield, p.stats, p.inFiles, p.up_limit, p.down_limit)
	p.unusedPeers.Remove(addr)
	go p.activePeers[addr.Value.(string)].PeerWriter()
	return
}

func (p *PeerMgr) AddIncomingPeer(c net.Conn) {
	if len(p.incomingPeers) >= INCOMING_PEERS {
		c.Close()
		return
	}
	addr := c.RemoteAddr().String()
	addr = addr[0:strings.Index(addr, ":")]
	// Check if peer has already connected
	// We should do this with peerId + ip, not only ip
	for p_addr, _ := range(p.incomingPeers) {
		if strings.HasPrefix(p_addr, addr) {
			log.Println("PeerMgr -> Incoming peer is already present")
			c.Close()
			return
		}
	}
	//log.Println("PeerMgr -> Adding incoming peer with address:", addr)
	p.incomingPeers[c.RemoteAddr().String()], _ = NewPeerFromConn(c, p.infohash, p.peerid, p.incoming, p.numPieces, p.requests, p.our_bitfield, p.stats, p.inFiles, p.up_limit, p.down_limit)
	go p.incomingPeers[c.RemoteAddr().String()].PeerWriter()
}
