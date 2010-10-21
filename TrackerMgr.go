package main

import(
	"log"
	"time"
	"strings"
	"strconv"
	"rand"
	"os"
	)


type TrackerMgr struct {
	// Chanels
	trackers map[string]*Tracker
	outPeerMgr chan <- peersList
	inPeerMgr  <- chan int
	outStatus chan <- *Status
	inStatus <- chan *Status
	announce *time.Ticker
	fromTracker chan peersList
	toTracker chan int
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
	bitfield *Bitfield
	pieceLength int64
}

func NewTrackerMgr(urls []string, infohash, port string, outPeerMgr chan peersList, inPeerMgr chan int, outStatus chan *Status, inStatus chan *Status, left int64, bitfield *Bitfield, pieceLength int64) (t *TrackerMgr) {
	sid := CLIENT_ID + "-" + strconv.Itoa(os.Getpid()) + strconv.Itoa64(rand.Int63())
	t = new(TrackerMgr)
	t.peerId = sid[0:20]
	t.trackers = make(map[string]*Tracker)
	t.inPeerMgr = inPeerMgr
	t.inStatus = inStatus
	t.outPeerMgr = outPeerMgr
	t.fromTracker = make(chan peersList)
	t.toTracker = make(chan int)
	t.num_peers = ACTIVE_PEERS + UNUSED_PEERS
	for _, url := range(urls) {
		if _, ok := t.trackers[url]; strings.HasPrefix(url, "http") && !ok {
			log.Println("TrackerMgr -> Creating new tracker:", url)
			t.trackers[url] = NewTracker(url, infohash, port, t.fromTracker, t.toTracker, left, bitfield, pieceLength, t.peerId)
			go t.trackers[url].Run()
		}
	}
	return
}

func (t *TrackerMgr) Run() {
	for {
		select {
			// Posible deadlock if PeerMgr tries to send num of peers
			// and we try to send peerList
			case num := <- t.inPeerMgr:
				//log.Println("TrackerMgr -> Requesting", num, "peers in new announce")
				t.num_peers = num
			case stat := <- t.inStatus:
				t.uploaded, t.downloaded = stat.uploaded, stat.downloaded
			case t.toTracker <- t.num_peers:
			case peers := <- t.fromTracker:
				t.num_peers -= peers.peers.Len()
				t.outPeerMgr <- peers
		}
	}
}