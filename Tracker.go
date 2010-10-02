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
	"rand"
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
	outPeerMgr chan <- peersList
	inPeerMgr  <- chan int
	outStatus chan <- *trackerStatusMsg
	inStatus <- chan *TrackerStatMsg
	announce *time.Ticker
	//inStatus		<- chan statusMsg
	// Internal data for tracker requests
	infohash, peerId, url, port, trackerId string
	interval, min_interval int64
	// Updated from the Status module
	uploaded, downloaded, left int64
	completed bool
	status string
	num_peers int
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

func NewTracker(url, infohash, port string, outPeerMgr chan peersList, inPeerMgr chan int, outStatus chan *trackerStatusMsg, inStatus chan *TrackerStatMsg, left int64) (t *Tracker) {
	sid := CLIENT_ID + "-" + strconv.Itoa(os.Getpid()) + strconv.Itoa64(rand.Int63())
	t = &Tracker{url: url, 
		infohash: infohash, 
		status: "started", 
		port: port, 
		peerId: sid[0:20], 
		outPeerMgr: outPeerMgr,
		inPeerMgr: inPeerMgr, 
		outStatus: outStatus,
		inStatus: inStatus,
		num_peers: ACTIVE_PEERS+UNUSED_PEERS,
		left: left,
		announce: time.NewTicker(1)}
	if t.left == 0 {
		t.completed = true
	}
	return
}

func (t *Tracker) Run() {
	for {
		select {
			case <- t.announce.C:
				if t.num_peers > 0 {
					log.Stderr("Tracker -> Requesting Tracker info")
					err := t.Request()
					if err != nil {
						log.Stderr("Tracker -> Error requesting Tracker info", err)
						t.announce.Stop()
						t.announce = time.NewTicker(TRACKER_ERR_INTERVAL)
					} else {
						log.Stderr("Tracker -> Requesting Tracker info finished OK, next announce:", t.interval)
						t.announce.Stop()
						t.announce = time.NewTicker(t.min_interval*NS_PER_S)
					}
				}
			case num := <- t.inPeerMgr:
				log.Stderr("Tracker -> Requesting", num, "peers in new announce")
				t.num_peers = num
			case stat := <- t.inStatus:
				t.uploaded, t.downloaded, t.left = stat.uploaded, stat.downloaded, stat.left
		}
	}
}

func (t *Tracker) Request() (err os.Error) {
	// Prepare request to make to the tracker
	if len(t.status) == 0 && !t.completed {
		if t.left == 0 {
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
		"&left=",http.URLEscape(strconv.Itoa64(t.left)),
		"&numwant=",http.URLEscape(strconv.Itoa(t.num_peers)),
		"&status=",http.URLEscape(t.status),
		"&compact=1")
		
	log.Stderr(url)
	
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
	t.num_peers = 0
	if len(tr.Tracker_id) > 0 {
		t.trackerId = tr.Tracker_id
	} 
	// Obtain new peers list
	msgPeers := peersList{peers: list.New()}
	
	for _, peer := range tr.Peers {
		msgPeers.peers.PushFront(fmt.Sprintf("%s:%d", peer.Ip, peer.Port))
	}
	// Send the new data to the PeerMgr process
	t.outPeerMgr <- msgPeers
	if t.status == "completed" {
		t.completed = true
	}
	t.status = ""
	
	// Obtain new stats about leechers/seeders
	/*msgStatus := trackerStatusMsg{
		FailureReason: tr.FailureReason, 
		WarningMessage: tr.WarningMessage, 
		Interval: tr.Interval, 
		TrackerId: tr.TrackerId, 
		Complete: tr.Complete, 
		Incomplete: tr.Incomplete}
	
	// Send the info to Status goroutine
	t.outStatus <- msgStatus
	*/
	return
}

