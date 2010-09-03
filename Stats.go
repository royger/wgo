// Keeps track of the speed of each peer
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package main

import(
	"log"
	"time"
	//"math"
	"fmt"
	)
	
type PeerStatMsg struct {
	size_up int64 // bytes
	size_down int64
	addr string
}

type SpeedInfo struct {
	/*upload int64
	download int64*/
	speed int64
	upload int64
	download int64
}

type TrackerStatMsg struct {
	uploaded, downloaded, left int64
}

type PeerStat struct {
	size_up int64 // bytes
	size_down int64
	pos int
	pod_up []int64
	pod_down []int64
}

type Stats struct {
	peers map[string] *PeerStat
	stats chan *PeerStatMsg
	inTracker chan <- *TrackerStatMsg
	inChokeMgr chan chan map[string]*SpeedInfo
	outPieceMgr chan string
	inPieceMgr chan *SpeedInfo
	left, size, uploaded, downloaded int64
	pod_up, pod_down []int64
	n int
}

func NewStats(stats chan *PeerStatMsg, inTracker chan *TrackerStatMsg, inChokeMgr chan chan map[string]*SpeedInfo, outPieceMgr chan string, inPieceMgr chan *SpeedInfo, left, size int64) (s *Stats) {
	s = new(Stats)
	s.peers = make(map[string] *PeerStat)
	s.stats = stats
	s.inTracker = inTracker
	s.inChokeMgr = inChokeMgr
	s.outPieceMgr = outPieceMgr
	s.inPieceMgr = inPieceMgr
	s.left = left
	s.size = size
	s.pod_up, s.pod_down = make([]int64, PONDERATION_TIME), make([]int64, PONDERATION_TIME)
	return
}

func (s *Stats) Update(msg *PeerStatMsg) {
	if _, ok := s.peers[msg.addr]; !ok {
		s.peers[msg.addr] = new(PeerStat)
		s.peers[msg.addr].pod_up = make([]int64, PONDERATION_TIME)
		s.peers[msg.addr].pod_down = make([]int64, PONDERATION_TIME)
	}
	s.peers[msg.addr].size_up += msg.size_up
	s.peers[msg.addr].size_down += msg.size_down
}

func (s *Stats) Remove(addr string) {
	if _, ok := s.peers[addr]; ok {
		s.peers[addr] = nil, false
	}
}

func (s *Stats) Round() {
	//log.Stderr("Stats -> Start processing stats")
	total_up := int64(0)
	total_down := int64(0)
	for _, peer := range(s.peers) {
		// Update global size
		total_up += peer.size_up
		total_down += peer.size_down
		// Update peer uploading/downloading ponderation
		peer.pod_up[peer.pos] = peer.size_up
		peer.pod_down[peer.pos] = peer.size_down
		peer.pos = (peer.pos+1)%PONDERATION_TIME
		// Reset counters
		peer.size_up = 0
		peer.size_down = 0
	}
	s.downloaded += total_up
	s.uploaded += total_down
	s.pod_up[s.n] = total_up
	s.pod_down[s.n] = total_down
	s.n = (s.n+1)%PONDERATION_TIME
	if s.left > 0 {
		s.left -= total_up
		if s.left < 0 {
			s.left = 0
		}
	}
	var ratio float64
	if s.uploaded == 0 {
		ratio = 0
	} else if s.downloaded == 0 {
		if s.left == 0 {
			ratio = float64(s.uploaded)/float64(s.size)
		}
	} else {
		ratio = float64(s.uploaded)/float64(s.downloaded)
	}
	total_up = 0
	total_down = 0
	for i := 0; i < PONDERATION_TIME; i++ {
		total_up += s.pod_up[i]
		total_down += s.pod_down[i]
	}
	total_up = total_up/PONDERATION_TIME
	total_down = total_down/PONDERATION_TIME
	log.Stderr("Stats -> Downloading speed:", total_up/1000, "KB/s Uploading Speed:", total_down/1000, "KB/s Left:", s.left/1000000, "MB Downloaded:", s.downloaded/1000000, "MB Uploaded:", s.uploaded/1000000, "MB Ratio:", fmt.Sprintf("%4.2f", ratio))
}

func (s *Stats) Run() {
	round := time.Tick(NS_PER_S)
	tracker := time.Tick(TRACKER_UPDATE*NS_PER_S)
	for {
		//log.Stderr("Stats -> Waiting for messages")
		select {
			case msg := <- s.stats:
				//log.Stderr("Stats -> Received message")
				if msg.size_up > 0 || msg.size_down > 0 {
					//log.Stderr("Stats -> Updating peer stats")
					s.Update(msg)
					//log.Stderr("Stats -> Finished updating peer stats")
				} else {
					//log.Stderr("Stats -> Removing peer from stats")
					s.Remove(msg.addr)
					//log.Stderr("Stats -> Finished removing peer")
				}
			case <- round:
				//log.Stderr("Stats -> Started processing stats")
				s.Round()
				//log.Stderr("Stats -> Finished processing stats")
			case <- tracker:
				s.inTracker <- &TrackerStatMsg{uploaded: s.uploaded, downloaded: s.downloaded, left: s.left}
			case c := <- s.inChokeMgr:
				peers := make(map[string]*SpeedInfo)
				for addr, peer := range(s.peers) {
					choke := new(SpeedInfo)
					if s.left == 0 {
						for _, speed := range peer.pod_down {
							choke.speed += speed
						}
					} else { 
						for _, speed := range peer.pod_up {
							choke.speed += speed
						}
					}
					choke.speed = choke.speed/PONDERATION_TIME
					peers[addr] = choke
				}
				c <- peers
			case addr := <- s.outPieceMgr:
				speed := new(SpeedInfo)
				if peer, ok := s.peers[addr]; ok {
					for _, up := range peer.pod_up {
						speed.upload += up
					}
					speed.upload = speed.upload/PONDERATION_TIME
				}
				s.inPieceMgr <- speed
		}
	}
}
