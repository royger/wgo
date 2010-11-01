// Management of a set of peers for a single torrent.
// Comunicates with the peers (readers/writers)
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package peers

import(
	"encoding/binary"
	"os"
	"log"
	"container/list"
	"net"
	"strings"
	"wgo/limiter"
	"wgo/bit_field"
	"wgo/files"
	"wgo/stats"
	"sync"
	)
	
const(
	UNUSED_PEERS = 200
	PERCENT_UNUSED_PEERS = 20
)

// We will use 1 channel to send the data from all peers (Readers)
// We will use separate channels to comunicate with the peers (Writers)
// So this implementation will use 1+num_peers channels


// General structure that will hold comunitacion channels
// and info about peers

type peerMgr struct {
	mutex *sync.Mutex
	activePeers map[string] *Peer // List of active peers
	incomingPeers map[string] *Peer // List of incoming connections
	unusedPeers *list.List
	pieceMgr PieceMgr
	stats stats.Stats
	our_bitfield *bit_field.Bitfield
	numPieces, lastPieceLength int64
	infohash, peerid string
	files files.Files
	l limiter.Limiter
}

type PeerMgr interface {
	DeletePeer(addr string)
	AddPeers(peers *list.List)
	AddPeer(conn net.Conn)
	GetPeers() (map[string]*Peer)
	SendHave(index int64)
	SendCancel(addr []string, index, begin, length int64)
	SetPieceMgr(pm PieceMgr)
	ActivePeers() int
	IncomingPeers() int
	UnusedPeers() int
	RequestPeers() int
}

func (p *peerMgr) DeletePeer(addr string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if peer, err := p.SearchPeer(addr); err == nil {
		p.Remove(peer)
	}
	return
}

func (p *peerMgr) AddPeers(peers *list.List) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	for i, addr := len(p.activePeers), peers.Front(); i < ACTIVE_PEERS && addr != nil; i, addr = i+1, peers.Front() {
		//log.Println("PeerMgr -> Adding Active Peer:", addr.Value.(string))
		if _, err := p.SearchPeer(addr.Value.(string)); err != nil {
			p.activePeers[addr.Value.(string)], err = NewPeer(addr.Value.(string), p.infohash, p.peerid, p, p.numPieces, p.lastPieceLength, p.pieceMgr, p.our_bitfield, p.stats, p.files, p.l)
			if err != nil {
				log.Println("PeerMgr -> Error creating peer:", err)
			}
			go p.activePeers[addr.Value.(string)].PeerWriter()
		}
		peers.Remove(addr)
	}
	p.unusedPeers.PushBackList(peers)
	//p.tracker <- peers
}

func (p *peerMgr) AddPeer(c net.Conn) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
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
	p.incomingPeers[c.RemoteAddr().String()], _ = NewPeerFromConn(c, p.infohash, p.peerid, p, p.numPieces, p.lastPieceLength, p.pieceMgr, p.our_bitfield, p.stats, p.files, p.l)
	go p.incomingPeers[c.RemoteAddr().String()].PeerWriter()
}

func (p *peerMgr) GetPeers() (peers map[string]*Peer) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	peers = make(map[string]*Peer)
	for addr, peer := range(p.activePeers) {
		peers[addr] = peer
	}
	for addr, peer := range(p.incomingPeers) {
		peers[addr] = peer
	}
	return
}

func (p *peerMgr) SendHave(index int64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	payLoad := make([]byte, 4)
	binary.BigEndian.PutUint32(payLoad[0:4], uint32(index))
	msg := &message{length: uint32(5), msgId: have, payLoad: payLoad}
	for _, peer := range(p.activePeers) {
		peer.incoming <- msg
	}
	for _, peer := range(p.incomingPeers) {
		peer.incoming <- msg
	}
}

func (p *peerMgr) SendCancel(addr []string, index, begin, length int64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	payLoad := make([]byte, 12)
	binary.BigEndian.PutUint32(payLoad[0:4], uint32(index))
	binary.BigEndian.PutUint32(payLoad[4:8], uint32(begin))
	binary.BigEndian.PutUint32(payLoad[8:12], uint32(length))
	msg := &message{length: uint32(13), msgId: cancel, payLoad: payLoad, addr: addr}
	for _, addr := range(addr) {
		if peer, ok := p.activePeers[addr]; ok {
			peer.incoming <- msg
		} else if peer, ok := p.incomingPeers[addr]; ok {
			peer.incoming <- msg
		}
	}
}

func (p *peerMgr) SetPieceMgr(pm PieceMgr) {
	p.pieceMgr = pm
}

func (p *peerMgr) ActivePeers() int {
	return len(p.activePeers)
}

func (p *peerMgr) IncomingPeers() int {
	return len(p.incomingPeers)
}

func (p *peerMgr) UnusedPeers() int {
	return p.unusedPeers.Len()
}

func (p *peerMgr) RequestPeers() int {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if p.unusedPeers.Len() == 0 {
		return (UNUSED_PEERS + (ACTIVE_PEERS - len(p.activePeers)))
	} else if ((p.unusedPeers.Len()*100)/UNUSED_PEERS) < PERCENT_UNUSED_PEERS {
		return (UNUSED_PEERS - p.unusedPeers.Len())
	}
	return 0
}
// Create a PeerMgr

func NewPeerMgr(numPieces int64, peerid, infohash string, our_bitfield *bit_field.Bitfield, st stats.Stats, fl files.Files, l limiter.Limiter, lastPieceLength int64) (pm PeerMgr, err os.Error) {
	p := new(peerMgr)
	p.mutex = new(sync.Mutex)
	p.numPieces = numPieces
	p.lastPieceLength = lastPieceLength
	p.infohash = infohash
	p.peerid = peerid
	p.activePeers = make(map[string] *Peer, ACTIVE_PEERS)
	p.incomingPeers = make(map[string] *Peer, INCOMING_PEERS)
	p.unusedPeers = list.New()
	//p.pieceMgr = pieceMgr
	p.our_bitfield = our_bitfield
	p.stats = st
	p.files = fl
	//p.up_limit = up_limit
	//p.down_limit = down_limit
	p.l = l
	pm = p
	return
}

// Search the peer

func (p *peerMgr) SearchPeer(addr string) (peer *Peer, err os.Error) {
	var ok bool
	if peer, ok = p.activePeers[addr]; ok {
		return
	}
	if peer, ok = p.incomingPeers[addr]; ok {
		return
	}
	return peer, os.NewError("PeerMgr -> Peer " + addr + " not found")
}

// Remove a peer

func (p *peerMgr) Remove(peer *Peer) {
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

func (p *peerMgr) AddNewPeer() (err os.Error) {
	addr := p.unusedPeers.Front()
	if addr == nil {
		// Requests new peers to the tracker module (check inactive peers & active peers also)
		//p.inTracker <- (UNUSED_PEERS + (ACTIVE_PEERS - len(p.activePeers)))
		return os.NewError("Unused peers list is empty")
	}
	// Check how much of the unsued peers list is used, and request more if needed
	/*if (p.unusedPeers.Len()/UNUSED_PEERS * 100) < PERCENT_UNUSED_PEERS {
		// request new peers to tracker
		p.inTracker <- (UNUSED_PEERS - p.unusedPeers.Len())
	}*/
	//log.Println("Adding Inactive Peer:", addr.Value.(string))
	p.activePeers[addr.Value.(string)], _ = NewPeer(addr.Value.(string), p.infohash, p.peerid, p, p.numPieces, p.lastPieceLength, p.pieceMgr, p.our_bitfield, p.stats, p.files, p.l)
	p.unusedPeers.Remove(addr)
	go p.activePeers[addr.Value.(string)].PeerWriter()
	return
}
