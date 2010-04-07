// Keeps track of the speed of each peer
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package main

import(
	"log"
	"time"
	)
	
type StatMsg struct {
	size_up int // bytes
	size_down int
	addr string
}

type PeerStat struct {
	size_up int // bytes
	size_down int
}

type Stats struct {
	peers map[string] *PeerStat
	stats chan *StatMsg
}

func NewStats(stats chan *StatMsg) (s *Stats) {
	s = new(Stats)
	s.peers = make(map[string] *PeerStat)
	s.stats = stats
	return
}

func (s *Stats) Update(msg *StatMsg) {
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
	total_up := 0
	total_down := 0
	for _, peer := range(s.peers) {
		total_up += peer.size_up
		total_down += peer.size_down
		peer.size_up = 0
		peer.size_down = 0
	}
	//log.Stderr("Stats -> Finished processing stats. Downloading speed:", total_up/1024, "KB/s Uploading Speed:", total_down/1024, "KB/s")
	log.Stderr("Stats -> Downloading speed:", total_up/1000, "KB/s Uploading Speed:", total_down/1000, "KB/s")
}

func (s *Stats) Run() {
	round := time.Tick(NS_PER_S)
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
		}
	}
}
