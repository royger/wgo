// Communication with the tracker.
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package tracker

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
	"wgo/bit_field"
	"encoding/binary"
	)
	
const(
	TRACKER_ERR_INTERVAL = 60
	DEFAULT_TRACKER_INTERVAL = 1200
	NS_PER_S = 1000000000
	ACTIVE_PEERS = 45
	UNUSED_PEERS = 200
)

// 1 channel to send new peers to peerMgr
// 1 channel to comunicate with the status goroutine
// 1 channel to receive the number of peers to ask for
// 1 channel to receive stats

type Tracker struct {
	// Chanels
	trackerMgr *TrackerMgr
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
	bitfield *bit_field.Bitfield
	pieceLength int64
	retry_time int64
}

// Struct to send data to the PeerMgr goroutine

/*type peersList struct {
	peers *list.List;
}*/

// Struct to send data to the Status goroutine

type trackerStatusMsg struct {
	FailureReason, WarningMessage, TrackerId string
	Complete, Incomplete, Interval int
}

func NewTracker(url, infohash, port string, tm *TrackerMgr, left int64, bf *bit_field.Bitfield, pieceLength int64, peerId string) (t *Tracker) {
	t = &Tracker{url: url, 
		infohash: infohash, 
		status: "started", 
		port: port, 
		peerId: peerId, 
		trackerMgr: tm,
		announce: time.NewTicker(1*NS_PER_S),
		bitfield: bf,
		pieceLength: pieceLength,
		retry_time: TRACKER_ERR_INTERVAL}
	if t.bitfield.Completed() {
		t.completed = true
	}
	return
}

func (t *Tracker) Run() {
	for {
		select {
			case <- t.announce.C:
				num_peers := t.trackerMgr.RequestPeers()
				//log.Println("Tracker -> Requesting", num_peers, "peers")
				if num_peers > 0 {
					t.uploaded, t.downloaded = t.trackerMgr.Stats()
					log.Println("Tracker -> Requesting Tracker info:", t.url)
					err := t.Request(num_peers)
					if err != nil {
						log.Println("Tracker -> Error requesting Tracker info", err, t.url)
						t.announce.Stop()
						t.announce = time.NewTicker(t.retry_time*NS_PER_S)
						t.retry_time *= 2
					} else {
						log.Println("Tracker -> Requesting Tracker info finished OK, next announce:", t.interval, t.url)
						t.retry_time = TRACKER_ERR_INTERVAL
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
	/*
	r, _, err := http.Get(url)
	if err != nil {
		return
	}
	defer r.Body.Close()
	if r.StatusCode >= 400 {
		data, _ := ioutil.ReadAll(r.Body)
		reason := "Bad Request " + string(data)
		log.Println(reason)
		err = os.NewError(reason)
		return
	}
	var tr2 TrackerResponse
	err = bencode.Unmarshal(r.Body, &tr2)
	r.Body.Close()
	if err != nil {
		return
	}
	tr = &tr2
	return
	*/
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
	var tr bencode.TrackerResponse
	err = bencode.Unmarshal(response.Body, &tr)
	if err != nil {
		return
	}
	t.interval = tr.Interval
	t.min_interval = tr.Min_interval
	if len(tr.Tracker_id) > 0 {
		t.trackerId = tr.Tracker_id
	} 
	// Obtain new peers list
	peers := list.New()
	
	//log.Println("Tracker -> Decoded", len(tr.Peers), "peers")
	for i := 0; i < len(tr.Peers); i = i+6 {
		peers.PushFront(fmt.Sprintf("%d.%d.%d.%d:%d", tr.Peers[i+0], tr.Peers[i+1], tr.Peers[i+2], tr.Peers[i+3], binary.BigEndian.Uint16([]byte(tr.Peers[i+4:i+6]))))
		//ip := fmt.Sprintf("%d.%d.%d.%d", peers[i+0], peers[i+1], peers[i+2], peers[i+3])
		//port := int64(binary.BigEndian.Uint16(peers[i+4:i+6]))
		//tracker.Peers = appendPeer(tracker.Peers, Peer{Ip: ip, Port: port})
	}
	/*for _, peer := range tr.Peers {
		peers.PushFront(fmt.Sprintf("%s:%d", peer.Ip, peer.Port))
	}*/
	//log.Println("Tracker -> Received", msgPeers.Len(), "peers")
	// Send the new data to the PeerMgr process
	t.trackerMgr.SavePeers(peers)
	if t.status == "completed" {
		t.completed = true
	}
	t.status = ""
	return
}

