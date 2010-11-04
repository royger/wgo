package choke

import(
	"sort"
	"log"
	"os"
	"time"
	"rand"
	"wgo/stats"
	"wgo/peers"
	)
	
const(
	CHOKE_ROUND = 10
	OPTIMISTIC_UNCHOKE = 30
	UPLOADING_PEERS = 5
	SNUBBED_PERIOD = 60
	NS_PER_S = 1000000000
)
	
type PeerChoke struct {
	am_choking, am_interested, peer_choking, peer_interested, snubbed bool
	unchoke bool
	speed int64
	peer *peers.Peer
}

type ChokeMgr struct {
	stats stats.Stats
	peerMgr peers.PeerMgr
	optimistic_unchoke int
}

type Speed []*PeerChoke

func NewChokeMgr(st stats.Stats, pm peers.PeerMgr) (c *ChokeMgr, err os.Error) {
	c = new(ChokeMgr)
	c.stats = st
	c.peerMgr = pm
	go c.Run()
	return
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
			interested = append(interested, peer)
		}
	}
	return
}

func SelectUninterested(peers []*PeerChoke) (uninterested []*PeerChoke) {
	uninterested = make([]*PeerChoke, 0, 10)
	for _, peer := range(peers) {
		if !peer.peer_interested {
			uninterested = append(uninterested, peer)
		}
	}
	return
}

func SelectChoked(peers []*PeerChoke) (choked []*PeerChoke) {
	choked = make([]*PeerChoke, 0, 10)
	for _, peer := range(peers) {
		if !peer.unchoke && peer.am_choking  {
			choked = append(choked, peer)
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
			peer.peer.Unchoke()
			//peer.incoming <- &message{length: 1, msgId: unchoke}
			num_unchoked++
		}
		if !peer.unchoke && !peer.am_choking {
			peer.peer.Choke()
			//peer.incoming <- &message{length: 1, msgId: choke}
			num_choked++
		}
	}
	//log.Println("ChokeMgr -> Unchoked", num_unchoked, "peers")
	//log.Println("ChokeMgr -> Choked", num_choked, "peers")
}

func (c *ChokeMgr) RequestPeers() []*PeerChoke {
	// Prepare peer array
	lastPiece := int64(0)
	// Request info
	//log.Println("ChokeMgr -> Receiving from channels")
	//c.inStats <- inStats
	stats := c.stats.GetStats()
	list := c.peerMgr.GetPeers()
	//log.Println("ChokeMgr -> Finished receiving")
	// Prepare peer array
	peers := make([]*PeerChoke, 0, 10)
	for addr, peer := range(list) {
		//log.Println("ChokeMgr -> Checking if completed")
		if peer.Connected() && !peer.Completed() {
			//log.Println("ChokeMgr -> Not completed, adding to list")
			p := new(PeerChoke)
			p.am_choking, p.am_interested, p.peer_choking, p.peer_interested, lastPiece = peer.Am_choking(), peer.Am_interested(), peer.Peer_choking(), peer.Peer_interested(), peer.LastPiece()
			now := time.Seconds()
			if ((now - lastPiece) > SNUBBED_PERIOD) && p.am_interested {
				p.snubbed = true
			}
			p.peer = peer
			if stat, ok := stats[addr]; ok {
				p.speed = stat.Speed
			}
			peers = append(peers, p)
		}
		//log.Println("ChokeMgr -> Finished processing peer")
	}
	//log.Println("ChokeMgr -> Returning list with len:", len(peers))
	return peers
}

func (c *ChokeMgr) Choking(peers []*PeerChoke) {
	//num_unchoked := 0
	c.optimistic_unchoke = (c.optimistic_unchoke+CHOKE_ROUND)%OPTIMISTIC_UNCHOKE
	speed := int64(0)
	// Get interested peers and sort by upload
	//log.Println("ChokeMgr -> Selecting interesting")
	//log.Println("ChokeMgr -> Finished selecting")
	if interested := SelectInterested(peers); len(interested) > 0 {
		//log.Println("ChokeMgr -> Sorting by upload")
		Sort(interested)
		//log.Println("ChokeMgr -> Finished sorting")
		// UnChoke peers starting by the one that has a higher upload speed an is interested
		// Reserve 1 slot for optimisting unchoking
		up_limit := UPLOADING_PEERS
		if c.optimistic_unchoke == 0 {
			up_limit--
		}
		for i := 0; i < len(interested) && i < up_limit; i++ {
			interested[i].unchoke = true
			speed = interested[i].speed
		}
	}
	// Unchoke peers which have a better upload rate than downloaders, but are not interested
	// Leave 1 slot for optimistic unchoking
	//log.Println("ChokeMgr -> Sorting by upload")
	if uninterested := SelectUninterested(peers); len(uninterested) > 0 {
		Sort(uninterested)
		for i := 0; i < len(uninterested) && speed <= uninterested[i].speed; i++ {
			uninterested[i].unchoke = true
		}
	}
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
	apply(peers)
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
	log.Println("ChokeMgr -> Choked peers:", num_choked, "Unchoked peers:", num_unchoked, "Total:", len(peers))
}

func (c *ChokeMgr) Run() {
	choking := time.Tick(CHOKE_ROUND*NS_PER_S)
	for {
		select {
			case <- choking:
				//log.Println("ChokeMgr -> Choke round")
				if peers := c.RequestPeers(); len(peers) > 0 {
					//log.Println("ChokeMgr -> Starting choke")
					c.Choking(peers)
					//c.Stats(peers)
					//log.Println("ChokeMgr -> Finished choke")
				}
				//log.Println("ChokeMgr -> Finished choke round")
		}
	}
}
