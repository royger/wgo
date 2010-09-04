package main

import(
	"sort"
	"log"
	"os"
	"time"
	"rand"
	)
	
type PeerChoke struct {
	am_choking, am_interested, peer_choking, peer_interested, snubbed bool
	unchoke bool
	speed int64
	incoming chan *message
}

type ChokeMgr struct {
	inStats chan chan map[string]*SpeedInfo
	inPeers chan chan map[string]*Peer
	optimistic_unchoke int
}

type Speed []*PeerChoke

func NewChokeMgr(inStats chan chan map[string]*SpeedInfo, inPeers chan chan map[string]*Peer) (c *ChokeMgr, err os.Error) {
	c = new(ChokeMgr)
	c.inStats = inStats
	c.inPeers = inPeers
	return
}


func appendPeer(slice []*PeerChoke, data *PeerChoke) []*PeerChoke {
	l := len(slice)
	if l + 1 > cap(slice) {  // reallocate
		// Allocate 10 more slots
		newSlice := make([]*PeerChoke, (l+10))
		// The copy function is predeclared and works for any slice type.
		copy(newSlice, slice)
		slice = newSlice
	}
	slice = slice[0:l+1]
	slice[l] = data
	return slice
}

func (l Speed) Len() int { return len(l) }
func (l Speed) Less(i, j int) bool { return l[i].speed < l[j].speed }
func (l Speed) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }

func Sort(list Speed) {
	sort.Sort(list)
}

func SelectInterested(peers []*PeerChoke) (interested []*PeerChoke) {
	interested = make([]*PeerChoke, 0, 10)
	for _, peer := range(peers) {
		if peer.peer_interested && !peer.snubbed {
			interested = appendPeer(interested, peer)
		}
	}
	return
}

func SelectUninterested(peers []*PeerChoke) (uninterested []*PeerChoke) {
	uninterested = make([]*PeerChoke, 0, 10)
	for _, peer := range(peers) {
		if !peer.peer_interested {
			uninterested = appendPeer(uninterested, peer)
		}
	}
	return
}

func SelectChoked(peers []*PeerChoke) (choked []*PeerChoke) {
	choked = make([]*PeerChoke, 0, 10)
	for _, peer := range(peers) {
		if !peer.unchoke && peer.am_choking  {
			choked = appendPeer(choked, peer)
		}
	}
	return
}

func apply(peers []*PeerChoke) {
	// Apply changes to peers
	num_unchoked := 0
	num_choked := 0
	for _, peer := range(peers) {
		if peer.unchoke && peer.am_choking {
			peer.incoming <- &message{length: 1, msgId: unchoke}
			num_unchoked++
		}
		if !peer.unchoke && !peer.am_choking {
			peer.incoming <- &message{length: 1, msgId: choke}
			num_choked++
		}
	}
	//log.Stderr("ChokeMgr -> Unchoked", num_unchoked, "peers")
	//log.Stderr("ChokeMgr -> Choked", num_choked, "peers")
}

func (c *ChokeMgr) RequestPeers() []*PeerChoke {
	// Prepare peer array
	inStats := make(chan map[string]*SpeedInfo)
	inPeers := make(chan map[string]*Peer)
	lastPiece := int64(0)
	// Request info
	//log.Stderr("ChokeMgr -> Receiving from channels")
	c.inStats <- inStats
	c.inPeers <- inPeers
	stats := <- inStats
	list := <- inPeers
	//log.Stderr("ChokeMgr -> Finished receiving")
	// Prepare peer array
	peers := make([]*PeerChoke, 0, 10)
	for addr, peer := range(list) {
		//log.Stderr("ChokeMgr -> Checking if completed")
		if peer.connected && !peer.bitfield.Completed() {
			//log.Stderr("ChokeMgr -> Not completed, adding to list")
			p := new(PeerChoke)
			p.am_choking, p.am_interested, p.peer_choking, p.peer_interested, lastPiece = peer.am_choking, peer.am_interested, peer.peer_choking, peer.peer_interested, peer.lastPiece
			now := time.Seconds()
			if ((now - lastPiece) > SNUBBED_PERIOD) && p.am_interested {
				p.snubbed = true
			}
			p.incoming = peer.incoming
			if stat, ok := stats[addr]; ok {
				p.speed = stat.speed
			}
			peers = appendPeer(peers, p)
		}
		//log.Stderr("ChokeMgr -> Finished processing peer")
	}
	//log.Stderr("ChokeMgr -> Returning list with len:", len(peers))
	return peers
}

func (c *ChokeMgr) Choking(peers []*PeerChoke) {
	//num_unchoked := 0
	c.optimistic_unchoke = (c.optimistic_unchoke+CHOKE_ROUND)%OPTIMISTIC_UNCHOKE
	speed := int64(0)
	// Get interested peers and sort by upload
	//log.Stderr("ChokeMgr -> Selecting interesting")
	//log.Stderr("ChokeMgr -> Finished selecting")
	if interested := SelectInterested(peers); len(interested) > 0 {
		//log.Stderr("ChokeMgr -> Sorting by upload")
		Sort(interested)
		//log.Stderr("ChokeMgr -> Finished sorting")
		// UnChoke peers starting by the one that has a higher upload speed an is interested
		// Reserve 1 slot for optimisting unchoking
		up_limit := UPLOADING_PEERS
		if c.optimistic_unchoke == 0 {
			up_limit--
		}
		for i := 0; i < len(interested) && i < up_limit; i++ {
			interested[i].unchoke = true
			//num_unchoked++
			speed = interested[i].speed
		}
	}
	// Unchoke peers which have a better upload rate than downloaders, but are not interested
	// Leave 1 slot for optimistic unchoking
	//log.Stderr("ChokeMgr -> Sorting by upload")
	if uninterested := SelectUninterested(peers); len(uninterested) > 0 {
		Sort(uninterested)
		for i := 0; i < len(uninterested) && speed <= uninterested[i].speed; i++ {
			uninterested[i].unchoke = true
		}
	}
	//Sort(peers)
	//log.Stderr(peers)
	//log.Stderr("ChokeMgr -> Finished sorting")
	/*for i := 0; i < len(peers) && speed <= peers[i].speed; i++ {
		peers[i].unchoke = true
	}*/
	// Apply changes to peers
	//log.Stderr("ChokeMgr -> Apply changes")
	if c.optimistic_unchoke == 0 {
		// Select 1 peer and unchoke it
		for choked := SelectChoked(peers); len(choked) > 0; choked = SelectChoked(peers) {
			n := rand.Intn(len(choked))
			choked[n].unchoke = true
			if choked[n].peer_interested {
				break
			}
		}
	}
	//choked := SelectChoked(peers)
	//log.Stderr("ChokeMgr ->", len(choked), "choked peers of", len(peers))
	apply(peers)
	//log.Stderr("ChokeMgr -> Finished applying")
	return
}

func (c *ChokeMgr) Stats(peers []*PeerChoke) {
	num_unchoked := 0
	num_choked := 0
	for _, peer := range(peers) {
		if peer.am_choking {
			num_choked++
		}
		if !peer.am_choking {
			num_unchoked++
		}
	}
	log.Stderr("ChokeMgr -> Choked peers:", num_choked, "Unchoked peers:", num_unchoked, "Total:", len(peers))
}

func (c *ChokeMgr) Run() {
	for {
		choking := time.Tick(CHOKE_ROUND*NS_PER_S)
		select {
			case <- choking:
				//log.Stderr("ChokeMgr -> Choke round")
				if peers := c.RequestPeers(); len(peers) > 0 {
					//log.Stderr("ChokeMgr -> Starting choke")
					c.Choking(peers)
					//c.Stats(peers)
					//log.Stderr("ChokeMgr -> Finished choke")
				}
				//log.Stderr("ChokeMgr -> Sending response to PeerMgr")
				c.inPeers <- nil
				//log.Stderr("ChokeMgr -> Finished choke round")
		}
	}
}
