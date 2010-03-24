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
	)

// We will use 1 channel to send the data from all peers (Readers)
// We will use separate channels to comunicate with the peers (Writers)
// So this implementation will use 1+num_peers channels


// General structure that will hold comunitacion channels
// and info about peers

type PeerMgr struct {
	incoming chan message // Shared channel where peer sends messages and PeerMgr receives
	activePeers map[string] *Peer // List of active peers
	inactivePeers map[string] *Peer // List of inactive peers
	unusedPeers *list.List
	tracker <- chan peersList // Channel used to comunicate the Tracker thread and the PeerMgr
	numPieces int64
	infohash, peerid string
}

// Create a PeerMgr

func NewPeerMgr(tracker chan peersList, numPieces int64, peerid, infohash string) (p *PeerMgr, err os.Error) {
	p = new(PeerMgr)
	p.incoming = make(chan message)
	p.tracker = tracker
	p.numPieces = numPieces
	p.infohash = infohash
	p.peerid = peerid
	p.activePeers = make(map[string] *Peer)
	p.inactivePeers = make(map[string] *Peer)
	p.unusedPeers = list.New()
	return
}

// Process messages from peers and do actions

func (p *PeerMgr) Run() {
	for {
		log.Stderr("Waiting for messages")
		select {
			case msg := <- p.incoming:
				log.Stderr("Processing Peer message")
				err := p.ProcessPeerMessage(msg)
				if err != nil {
					log.Stderr(err)
				}
				log.Stderr("Finished processing Peer message")
			case msg := <- p.tracker:
				log.Stderr("Processing Tracker list")
				p.ProcessTrackerMessage(msg)
				log.Stderr("Finished processing Tracker list. Active peers:", len(p.activePeers), "Inactive peers:", len(p.inactivePeers))
		}
	}
}

func (p *PeerMgr) ProcessPeerMessage(msg message) (err os.Error) {
	//log.Stderr("Searching peer...")
	peer, err := p.SearchPeer(msg.addr)
	//log.Stderr("Searching peer finished")
	if err != nil {
		//log.Stderr("Peer not found")
		return os.NewError("Peer not found")
	}
	log.Stderr("Message:", msg)
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

// Tracker sends us new peers

func (p *PeerMgr) ProcessTrackerMessage(msg peersList) {
	// See if activePeers list is not full
	for i, addr := len(p.activePeers), msg.peers.Front(); i < ACTIVE_PEERS && addr != nil; i, addr = i+1, msg.peers.Front() {
		outgoing := make(chan message)
		log.Stderr("Adding Active Peer:", addr.Value.(string))
		p.activePeers[addr.Value.(string)], _ = NewPeer(addr.Value.(string), p.infohash, p.peerid, outgoing, p.incoming, p.numPieces)
		go p.activePeers[addr.Value.(string)].PeerWriter()
		msg.peers.Remove(addr)
	}
	// See if inactivePeers list is not full
	for i, addr := len(p.inactivePeers), msg.peers.Front(); i < INACTIVE_PEERS && addr != nil; i, addr = i+1,msg.peers.Front() {
		log.Stderr("Adding Inactive Peer:", addr.Value.(string))
		outgoing := make(chan message)
		p.inactivePeers[addr.Value.(string)], _ = NewPeer(addr.Value.(string), p.infohash, p.peerid, outgoing, p.incoming, p.numPieces)
		go p.inactivePeers[addr.Value.(string)].PeerWriter()
		msg.peers.Remove(addr)
	}
	// Add remaining peers to the unused list
	p.unusedPeers.PushBackList(msg.peers)
}

// Search the peer

func (p *PeerMgr) SearchPeer(addr string) (peer *Peer, err os.Error) {
	peer, present := p.activePeers[addr]
	if present {
		return
	}
	peer, present = p.inactivePeers[addr]
	if present {
		return
	}
	return peer, os.NewError("Peer not found")
}

// Remove a peer

func (p *PeerMgr) Remove(peer *Peer) {
	peer.Close()
	_, present := p.activePeers[peer.addr]
	if present {
		p.activePeers[peer.addr] = peer, false
		p.AddNewActivePeer()
		return
	}
	_, present = p.inactivePeers[peer.addr]
	if present {
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

func (p *PeerMgr) AddNewInactivePeer() (err os.Error){
	addr := p.unusedPeers.Front()
	if addr == nil {
		return os.NewError("Unused peers list is empty")
	}
	log.Stderr("Adding Inactive Peer:", addr.Value.(string))
	outgoing := make(chan message)
	p.inactivePeers[addr.Value.(string)], _ = NewPeer(addr.Value.(string), p.infohash, p.peerid, outgoing, p.incoming, p.numPieces)
	p.unusedPeers.Remove(addr)
	go p.inactivePeers[addr.Value.(string)].PeerWriter()
	return
}

