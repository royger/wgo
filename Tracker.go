// Communication with the tracker.
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package main

import(
	"http"
	"strconv"
	"log"
	"os"
	"fmt"
	"io/ioutil"
	"container/list"
	"time"
	"wgo/bencode"
	)

// 1 channel to send new peers to peerMgr
// 1 channel to comunicate with the status goroutine
// 1 channel to receive the number of peers to ask for
// 1 channel to receive stats

type Tracker struct {
	// Chanels
	toTrackerMgr chan <- peersList
	fromTrackerMgr  <- chan int
	announce *time.Ticker
	//inStatus		<- chan statusMsg
	// Internal data for tracker requests
	infohash, peerId, url, port, trackerId string
	interval, min_interval int64
	// Updated from the Status module
	uploaded, downloaded int64
	completed bool
	status string
	// Bitfield
	bitfield *Bitfield
	pieceLength int64
}

// Struct to send data to the PeerMgr goroutine

type peersList struct {
	peers *list.List;
}

// Struct to send data to the Status goroutine

type trackerStatusMsg struct {
	FailureReason, WarningMessage, TrackerId string
	Complete, Incomplete, Interval int
}

func NewTracker(url, infohash, port string, toTrackerMgr chan peersList, fromTrackerMgr chan int, left int64, bitfield *Bitfield, pieceLength int64, peerId string) (t *Tracker) {
	t = &Tracker{url: url, 
		infohash: infohash, 
		status: "started", 
		port: port, 
		peerId: peerId, 
		toTrackerMgr: toTrackerMgr,
		fromTrackerMgr: fromTrackerMgr, 
		announce: time.NewTicker(1),
		bitfield: bitfield,
		pieceLength: pieceLength}
	if t.bitfield.Completed() {
		t.completed = true
	}
	return
}

func (t *Tracker) Run() {
	for {
		select {
			case <- t.announce.C:
				num_peers := <- t.fromTrackerMgr
				if num_peers > 0 {
					log.Println("Tracker -> Requesting Tracker info:", t.url)
					err := t.Request(num_peers)
					if err != nil {
						log.Println("Tracker -> Error requesting Tracker info", err, t.url)
						t.announce.Stop()
						t.announce = time.NewTicker(TRACKER_ERR_INTERVAL*NS_PER_S)
					} else {
						log.Println("Tracker -> Requesting Tracker info finished OK, next announce:", t.interval, t.url)
						t.announce.Stop()
						if t.min_interval > 0 {
							t.announce = time.NewTicker(t.min_interval*NS_PER_S)
						} else if t.interval > 0 {
							t.announce = time.NewTicker(t.interval*NS_PER_S)
						} else {
							t.announce = time.NewTicker(DEFAULT_TRACKER_INTERVAL*NS_PER_S)
						}
					}
				}
		}
	}
}

func (t *Tracker) Request(num_peers int) (err os.Error) {
	// Prepare request to make to the tracker
	left := (t.bitfield.Len() - t.bitfield.Count())*t.pieceLength
	if len(t.status) == 0 && !t.completed {
		if left == 0 {
			t.status = "completed"
		}
	}
	url:= fmt.Sprint(t.url,
		"?",
		"info_hash=",http.URLEscape(t.infohash),
		"&peer_id=",http.URLEscape(t.peerId),
		"&port=",http.URLEscape(t.port),
		"&uploaded=",http.URLEscape(strconv.Itoa64(t.uploaded)),
		"&downloaded=",http.URLEscape(strconv.Itoa64(t.downloaded)),
		"&left=",http.URLEscape(strconv.Itoa64(left)),
		"&numwant=",http.URLEscape(strconv.Itoa(num_peers)),
		"&status=",http.URLEscape(t.status),
		"&compact=1")
	
	if len(t.trackerId) > 0 {
		url += "&tracker_id=" + http.URLEscape(t.trackerId)
	}

	response, _, err := http.Get(url)
	if err != nil { return }
	defer response.Body.Close()
	
	// Check if request was succesful
	if response.StatusCode != http.StatusOK {
		data, _ := ioutil.ReadAll(response.Body)
		reason := "Bad Request " + string(data)
		err = os.NewError(reason)
		return
	}
	
	// Create new TrackerResponse and decode the data
	var tr *bencode.Tracker
	tr, err = bencode.ParseTracker(response.Body)
	if err != nil {
		return
	}
	t.interval = tr.Interval
	t.min_interval = tr.Min_interval
	if len(tr.Tracker_id) > 0 {
		t.trackerId = tr.Tracker_id
	} 
	// Obtain new peers list
	msgPeers := peersList{peers: list.New()}
	
	for _, peer := range tr.Peers {
		msgPeers.peers.PushFront(fmt.Sprintf("%s:%d", peer.Ip, peer.Port))
	}
	// Send the new data to the PeerMgr process
	t.toTrackerMgr <- msgPeers
	if t.status == "completed" {
		t.completed = true
	}
	t.status = ""
	return
}

