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

type Tracker struct {
	// Chanels
	outPeerMgr chan <- peersList
	outStatus chan <- trackerStatusMsg
	announce <- chan int64
	//inStatus		<- chan statusMsg
	// Internal data for tracker requests
	infohash, peerId, url, port string
	interval int
	// Updated from the Status module
	uploaded, downloaded, left int
	status string
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

// Struct to save the info returned by the tracker

type TrackerResponse struct {
	FailureReason  string "failure reason"
	WarningMessage string "warning message"
	Interval       int
	MinInterval    int    "min interval"
	TrackerId      string "tracker id"
	Complete       int
	Incomplete     int
	Peers          string
}

func NewTracker(url, infohash, port string, outPeerMgr chan peersList, outStatus chan trackerStatusMsg) (t *Tracker) {
	sid := CLIENT_ID + "-" + strconv.Itoa(os.Getpid()) + strconv.Itoa64(rand.Int63())
	t = &Tracker{url: url, 
		infohash: infohash, 
		status: "started", 
		port: port, 
		peerId: sid[0:20], 
		outPeerMgr: outPeerMgr, 
		outStatus: outStatus,
		announce: time.Tick(TRACKER_ERR_INTERVAL)}
	return
}

func (t *Tracker) Run() {
	for {
		select {
			case <- t.announce:
				log.Stderr("Tracker -> Requesting Tracker info")
				err := t.Request()
				if err != nil {
					log.Stderr("Tracker -> Error requesting Tracker info", err)
					t.announce = time.Tick(TRACKER_ERR_INTERVAL)
				} else {
					log.Stderr("Tracker -> Requesting Tracker info finished OK, next announce:", t.interval)
					t.announce = time.Tick(int64(t.interval)*NS_PER_S)
				}
		}
	}
}

func (t *Tracker) Request() (err os.Error) {
	// Prepare request to make to the tracker
	
	url:= fmt.Sprint(t.url,
		"?",
		"info_hash=",http.URLEscape(t.infohash),
		"&peer_id=",http.URLEscape(t.peerId),
		"&port=",http.URLEscape(t.port),
		"&uploaded=",strconv.Itoa(t.uploaded),
		"&downloaded=",strconv.Itoa(t.downloaded),
		"&left=",strconv.Itoa(t.left),
		"&numwant=",NUM_PEERS,
		"&status=",http.URLEscape(t.status))
		
	log.Stderr(url)

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
	var tr TrackerResponse
	err = bencode.Unmarshal(response.Body, &tr)
	if err != nil {
		return
	}
	t.interval = tr.Interval
	// Obtain new peers list
	msgPeers := peersList{peers: list.New()}
		
	for i := 0; i < len(tr.Peers); i += 6 {
		//log.Stderr(fmt.Sprintf("%d.%d.%d.%d:%d", tr.Peers[i+0], tr.Peers[i+1], tr.Peers[i+2], tr.Peers[i+3], (uint16(tr.Peers[i+4])<<8)|uint16(tr.Peers[i+5])))
		msgPeers.peers.PushFront(fmt.Sprintf("%d.%d.%d.%d:%d", tr.Peers[i+0], tr.Peers[i+1], tr.Peers[i+2], tr.Peers[i+3], (uint16(tr.Peers[i+4])<<8)|uint16(tr.Peers[i+5])))
	}
	
	//log.Stderr("Peer List size:", msgPeers.peers.Len())
	// Send the new data to the PeerMgr process
	t.outPeerMgr <- msgPeers
	
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

