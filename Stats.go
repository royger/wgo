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

type TrackerStatMsg struct {
	uploaded, downloaded, left int64
}

type PeerStat struct {
	size_up int64 // bytes
	size_down int64
}

type Stats struct {
	peers map[string] *PeerStat
	stats chan *PeerStatMsg
	inTracker chan <- *TrackerStatMsg
	left, size, uploaded, downloaded int64
}

func NewStats(stats chan *PeerStatMsg, inTracker chan *TrackerStatMsg, left, size int64) (s *Stats) {
	s = new(Stats)
	s.peers = make(map[string] *PeerStat)
	s.stats = stats
	s.inTracker = inTracker
	s.left = left
	s.size = size
	return
}

func (s *Stats) Update(msg *PeerStatMsg) {
	if _, ok := s.peers[msg.addr]; !ok {
		s.peers[msg.addr] = new(PeerStat)
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
		total_up += peer.size_up
		total_down += peer.size_down
		peer.size_up = 0
		peer.size_down = 0
	}
	//log.Stderr("Stats -> Finished processing stats. Downloading speed:", total_up/1024, "KB/s Uploading Speed:", total_down/1024, "KB/s")
	s.downloaded += total_up
	s.uploaded += total_down
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
		}
	}
}
