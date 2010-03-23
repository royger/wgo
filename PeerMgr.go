// Management of a set of peers for a single torrent.
// Comunicates with the peers (readers/writers)
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package main

import(
	"os"
	"encoding/binary"
	"log"
	"time"
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
	return
}

// Process messages from peers and do actions

func (p *PeerMgr) Run() {
	go p.KeepAlive()
	for {
		log.Stderr("Waiting for messages")
		select {
			case msg := <- p.incoming:
				log.Stderr("Processing Peer message")
				p.ProcessPeerMessage(msg)
				log.Stderr("Finished processing Peer message")
			case msg := <- p.tracker:
				log.Stderr("Processing Tracker list")
				p.ProcessTrackerMessage(msg)
				log.Stderr("Finished processing Tracker list. Active peers:", len(p.activePeers), "Inactive peers:", len(p.inactivePeers))
		}
	}
}

func (p *PeerMgr) ProcessPeerMessage(msg message) {
	//log.Stderr("Searching peer...")
	peer, err := p.SearchPeer(msg.addr)
	//log.Stderr("Searching peer finished")
	if err != nil {
		//log.Stderr("Peer not found")
		return
	}
	log.Stderr("Message:", msg)
	if msg.length == 0 {
		// Keep-alive message
		peer.received_keepalive = time.Seconds()
		return
	}
	switch msg.msgId {
		case choke:
			// Choke peer
			peer.peer_choking = true
			log.Stderr("Peer", peer.addr, "choked")
		case unchoke:
			// Unchoke peer
			peer.peer_choking = false
			log.Stderr("Peer", peer.addr, "unchoked")
		case interested:
			// Mark peer as interested
			peer.peer_interested = true
			log.Stderr("Peer", peer.addr, "interested")
		case uninterested:
			// Mark peer as uninterested
			peer.peer_interested = false
			log.Stderr("Peer", peer.addr, "uninterested")
		case have:
			// Update peer bitfield
			peer.bitfield.Set(int(binary.BigEndian.Uint32(msg.payLoad)))
			log.Stderr("Peer", peer.addr, "have")
		case bitfield:
			// Set peer bitfield
			peer.bitfield, err = NewBitfieldFromBytes(int(p.numPieces), msg.payLoad)
			if err != nil {
				p.Remove(peer)
			}
			log.Stderr("Peer", peer.addr, "bitfield")
		case request:
			// Peer requests a block
		case piece:
			// We have received a piece
		case cancel:
			// Cancel a previous request
		case port:
			// DHT stuff
		case exit:
			// Internal message used to remove a peer
			log.Stderr("Removing peer", peer.addr)
			p.Remove(peer)
			log.Stderr("Peer", peer.addr, "removed")
	}
}

// Tracker sends us new peers

func (p *PeerMgr) ProcessTrackerMessage(msg peersList) {
	// See if activePeers list is not full
	for i, addr := len(p.activePeers), msg.peers.Front(); i < ACTIVE_PEERS && addr != nil; i, addr = i+1, msg.peers.Front() {
		outgoing := make(chan message)
		log.Stderr("Adding Active Peer:", addr.Value.(string))
		p.activePeers[addr.Value.(string)], _ = NewPeer(addr.Value.(string), p.infohash, p.peerid, outgoing, p.incoming)
		go p.activePeers[addr.Value.(string)].PeerWriter()
		msg.peers.Remove(addr)
	}
	// See if inactivePeers list is not full
	for i, addr := len(p.inactivePeers), msg.peers.Front(); i < INACTIVE_PEERS && addr != nil; i, addr = i+1,msg.peers.Front() {
		log.Stderr("Adding Inactive Peer:", addr.Value.(string))
		outgoing := make(chan message)
		p.inactivePeers[addr.Value.(string)], _ = NewPeer(addr.Value.(string), p.infohash, p.peerid, outgoing, p.incoming)
		go p.inactivePeers[addr.Value.(string)].PeerWriter()
		msg.peers.Remove(addr)
	}
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
		// TODO: Add a new inactive peer
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
		// TODO: Add a new peer to the inactivePeers list
}

func (p *PeerMgr) KeepAlive() {
	for {
		log.Stderr("Starting to send keep-alive messages")
		actual := time.Seconds()
		for _, peer := range p.activePeers {
			if peer.last_activity != 0 && (actual - peer.last_activity)*NS_PER_S > KEEP_ALIVE_MSG {
				log.Stderr("Sending keep-alive to", peer.addr)
				_ = peer.incoming <- message{length: 0}
			}
		}
		for _, peer := range p.inactivePeers {
			if peer.last_activity != 0 && (actual - peer.last_activity)*NS_PER_S > KEEP_ALIVE_MSG {
				log.Stderr("Sending keep-alive to", peer.addr)
				_ = peer.incoming <- message{length: 0}
			}
		}
		log.Stderr("Finished sending keep-alive messages")
		time.Sleep(KEEP_ALIVE_ROUND)
	}
}

