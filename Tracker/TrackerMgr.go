// Communication with the tracker.
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package tracker

import(
	"log"
	"strings"
	"wgo/bit_field"
	"wgo/stats"
	"container/list"
	"wgo/peers"
	)


type TrackerMgr struct {
	// Chanels
	trackers map[string]*Tracker
	//outPeerMgr chan <- *list.List
	peerMgr peers.PeerMgr
	// outStatus chan <- *Status
	stats stats.Stats
	//stats stats.Stats
	//inStatus		<- chan statusMsg
	// Internal data for tracker requests
	infohash, peerId, url, port, trackerId string
	interval, min_interval int64
	// Updated from the Status module
	uploaded, downloaded, left int64
	completed bool
	status string
	num_peers int
	// Bitfield
	bitfield *bit_field.Bitfield
	pieceLength int64
}

func (t *TrackerMgr) RequestPeers() int {
	return t.peerMgr.RequestPeers()
}

func (t *TrackerMgr) Stats() (int64, int64) {
	return t.stats.GetGlobalStats()
}

func (t* TrackerMgr) SavePeers(peers *list.List) {
	t.peerMgr.AddPeers(peers)
}

func NewTrackerMgr(urls []string, infohash, port string, peerMgr peers.PeerMgr, left int64, bf *bit_field.Bitfield, pieceLength int64, peerId string, s stats.Stats) (t *TrackerMgr) {
	//sid := CLIENT_ID + "-" + strconv.Itoa(os.Getpid()) + strconv.Itoa64(rand.Int63())
	t = new(TrackerMgr)
	t.peerId = peerId
	t.trackers = make(map[string]*Tracker)
	//t.outPeerMgr = outPeerMgr
	t.peerMgr = peerMgr
	t.stats = s
	t.num_peers = ACTIVE_PEERS + UNUSED_PEERS
	for _, url := range(urls) {
		if _, ok := t.trackers[url]; strings.HasPrefix(url, "http") && !ok {
			log.Println("TrackerMgr -> Creating new tracker:", url)
			t.trackers[url] = NewTracker(url, infohash, port, t, left, bf, pieceLength, t.peerId)
			go t.trackers[url].Run()
		}
	}
	return
}
