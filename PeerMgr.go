// Management of a set of peers for a single torrent.
// Comunicates with the peers (readers/writers)
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package main

import(
	"os"
	//"encoding/binary"
	"log"
	"container/list"
	"time"
	)

// We will use 1 channel to send the data from all peers (Readers)
// We will use separate channels to comunicate with the peers (Writers)
// So this implementation will use 1+num_peers channels


// General structure that will hold comunitacion channels
// and info about peers

type PeerMgr struct {
	incoming chan *message // Shared channel where peer sends messages and PeerMgr receives
	peerMgr chan *message
	activePeers map[string] *Peer // List of active peers
	inactivePeers map[string] *Peer // List of inactive peers
	unusedPeers *list.List
	tracker <- chan peersList // Channel used to comunicate the Tracker thread and the PeerMgr
	requests chan *PieceMgrRequest
	stats chan *StatMsg
	our_bitfield *Bitfield
	numPieces int64
	infohash, peerid string
}

// Create a PeerMgr

func NewPeerMgr(tracker chan peersList, numPieces int64, peerid, infohash string, requests chan *PieceMgrRequest, peerMgr chan *message, our_bitfield *Bitfield, stats chan *StatMsg) (p *PeerMgr, err os.Error) {
	p = new(PeerMgr)
	p.incoming = make(chan *message)
	p.tracker = tracker
	p.numPieces = numPieces
	p.infohash = infohash
	p.peerid = peerid
	p.activePeers = make(map[string] *Peer, ACTIVE_PEERS)
	p.inactivePeers = make(map[string] *Peer, INACTIVE_PEERS)
	p.unusedPeers = list.New()
	p.requests = requests
	p.our_bitfield = our_bitfield
	p.peerMgr = peerMgr
	p.stats = stats
	return
}

// Process messages from peers and do actions

func (p *PeerMgr) Run() {
	chokeRound := time.Tick(10*NS_PER_S)
	for {
		log.Stderr("PeerMgr -> Waiting for messages")
		select {
			case msg := <- p.incoming:
				log.Stderr("PeerMgr -> Processing Peer message")
				log.Stderr(msg)
				err := p.ProcessPeerMessage(msg)
				if err != nil {
					log.Stderr(err)
				}
				log.Stderr("PeerMgr -> Finished processing Peer message")
			case msg := <- p.tracker:
				log.Stderr("PeerMgr -> Processing Tracker list")
				p.ProcessTrackerMessage(msg)
				log.Stderr("PeerMgr -> Finished processing Tracker list. Active peers:", len(p.activePeers), "Inactive peers:", len(p.inactivePeers))
			case <- chokeRound:
				log.Stderr("PeerMgr -> Unchoking peers")
				err := p.UnchokePeers()
				if err != nil {
					log.Stderr("Error unchoking peers")
				}
				log.Stderr("PeerMgr -> Finished unchoking peers")
			case msg := <- p.peerMgr:
				log.Stderr("PeerMgr -> Broadcasting message")
				// Broadcast have message
				p.Broadcast(msg)
				log.Stderr("PeerMgr -> Finished broadcasting peer message")
		}
	}
}

func (p *PeerMgr) ProcessPeerMessage(msg *message) (err os.Error) {
	//log.Stderr("Searching peer...")
	peer, err := p.SearchPeer(msg.addr[0])
	//log.Stderr("Searching peer finished")
	if err != nil {
		//log.Stderr("Peer not found")
		return
	}
	//log.Stderr("Message:", msg)
	switch msg.msgId {
		case exit:
			// Internal message used to remove a peer
			log.Stderr("Removing peer", peer.addr)
			p.Remove(peer)
			log.Stderr("Peer", peer.addr, "removed")
		default:
			log.Stderr("Unknown message ID")
	}
	return
}

// Broadcast message to all peers

func (p *PeerMgr) Broadcast(msg *message) {
	if len(msg.addr) == 0 {
		// Broadcast message to all peers
		//log.Stderr("Broadcasting have message")
		for _, peer := range(p.activePeers) {
			peer.incoming <- msg
		}
		for _, peer := range(p.activePeers) {
			peer.incoming <- msg
		}
		//log.Stderr("Finished broadcasting have message")
	} else {
		// Send message to some peers only
		for _, addr := range(msg.addr) {
			if peer, ok := p.activePeers[addr]; ok {
				peer.incoming <- msg
			} else if peer, ok := p.inactivePeers[addr]; ok {
				peer.incoming <- msg
			}
		}
	}
}

// Tracker sends us new peers

func (p *PeerMgr) ProcessTrackerMessage(msg peersList) {
	// See if activePeers list is not full
	for i, addr := len(p.activePeers), msg.peers.Front(); i < ACTIVE_PEERS && addr != nil; i, addr = i+1, msg.peers.Front() {
		//log.Stderr("Adding Active Peer:", addr.Value.(string))
		p.activePeers[addr.Value.(string)], _ = NewPeer(addr.Value.(string), p.infohash, p.peerid, p.incoming, p.numPieces, p.requests, p.our_bitfield, p.stats)
		go p.activePeers[addr.Value.(string)].PeerWriter()
		msg.peers.Remove(addr)
	}
	// See if inactivePeers list is not full
	for i, addr := len(p.inactivePeers), msg.peers.Front(); i < INACTIVE_PEERS && addr != nil; i, addr = i+1,msg.peers.Front() {
		//log.Stderr("Adding Inactive Peer:", addr.Value.(string))
		p.inactivePeers[addr.Value.(string)], _ = NewPeer(addr.Value.(string), p.infohash, p.peerid, p.incoming, p.numPieces, p.requests, p.our_bitfield, p.stats)
		go p.inactivePeers[addr.Value.(string)].PeerWriter()
		msg.peers.Remove(addr)
	}
	// Add remaining peers to the unused list
	p.unusedPeers.PushBackList(msg.peers)
}

// Search the peer

func (p *PeerMgr) SearchPeer(addr string) (peer *Peer, err os.Error) {
	var ok bool
	if peer, ok = p.activePeers[addr]; ok {
		return
	}
	if peer, ok = p.inactivePeers[addr]; ok {
		return
	}
	return peer, os.NewError("Peer not found")
}

// Remove a peer

func (p *PeerMgr) Remove(peer *Peer) {
	//peer.Close()
	if _, ok := p.activePeers[peer.addr]; ok {
		p.activePeers[peer.addr] = peer, false
		p.AddNewActivePeer()
		return
	}
	if _, ok := p.inactivePeers[peer.addr]; ok {
		p.inactivePeers[peer.addr] = peer, false
		err := p.AddNewInactivePeer()
		if err != nil {
			log.Stderr(err)
		}
		return
	}
}

// Add a new peer to the activePeers map

func (p *PeerMgr) AddNewActivePeer() {
	// See if we can find a peer that is not choking and we are interested
	for addr, peer := range p.inactivePeers {
		if !peer.peer_choking && peer.am_interested {
			p.activePeers[addr] = peer
			p.inactivePeers[addr] = peer, false
			goto exit
		}
	}
	// See if we can find a peer in which we are interested
	for addr, peer := range p.inactivePeers {
		if peer.am_interested {
			p.activePeers[addr] = peer
			p.inactivePeers[addr] = peer, false
			goto exit
		}
	}
	// Add any peer
	for addr, peer := range p.inactivePeers {
		p.activePeers[addr] = peer
		p.inactivePeers[addr] = peer, false
		goto exit
	}
exit:
	err := p.AddNewInactivePeer()
	if err != nil {
		log.Stderr(err)
	}
}

func (p *PeerMgr) AddNewInactivePeer() (err os.Error) {
	addr := p.unusedPeers.Front()
	if addr == nil {
		return os.NewError("Unused peers list is empty")
	}
	//log.Stderr("Adding Inactive Peer:", addr.Value.(string))
	p.inactivePeers[addr.Value.(string)], _ = NewPeer(addr.Value.(string), p.infohash, p.peerid, p.incoming, p.numPieces, p.requests, p.our_bitfield, p.stats)
	p.unusedPeers.Remove(addr)
	go p.inactivePeers[addr.Value.(string)].PeerWriter()
	return
}

// Unchoke active peers
// This will be implemented correctly in the
// ChokeMgr, but for now we unchoke interested peers
// That are on the activePeers array

func (p *PeerMgr) UnchokePeers() (err os.Error) {
	for _, peer := range(p.activePeers) {
		if peer.peer_interested && peer.am_choking {
			peer.incoming <- &message{length: 1, msgId: unchoke}
		}
	}
	return
}
