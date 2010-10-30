// Keeps track of the speed of each peer
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package stats

import(
	"log"
	"time"
	//"math"
	"fmt"
	"wgo/bit_field"
	"sync"
	)
	
const(
	NS_PER_S = 1000000000
	PONDERATION_TIME = 10 // in seconds
	TRACKER_UPDATE = 60
)

type Status struct {
	Uploaded, Downloaded, Speed int64
	Addr string
}

type PeerStat struct {
	size_up int64 // bytes
	size_down int64
	pos int
	pod_up []int64
	pod_down []int64
}

type stats struct {
	mutex *sync.Mutex
	peers map[string] *PeerStat
	size, uploaded, downloaded int64
	pod_up, pod_down []int64
	n int
	bitfield *bit_field.Bitfield
	pieceLength int64
}

type Stats interface {
	Update(addr string, uploaded, downloaded int64)
	GetStats() (map[string]*Status)
	GetSpeed(addr string) (speed int64)
	GetGlobalStats() (uploaded, downloaded int64)
}

func (s *stats) Update(addr string, uploaded, downloaded int64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if uploaded > 0 || downloaded > 0 {
		//log.Println("Stats -> Updating peer stats")
		s.update(addr, uploaded, downloaded)
		//log.Println("Stats -> Finished updating peer stats")
	} else {
		//log.Println("Stats -> Removing peer from stats")
		s.remove(addr)
		//log.Println("Stats -> Finished removing peer")
	}
}

func (s *stats) GetStats() (map[string]*Status) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	peers := make(map[string]*Status)
	for addr, peer := range(s.peers) {
		choke := new(Status)
		// use bitfield instead of "left"
		if s.bitfield.Completed() {
			for _, speed := range peer.pod_down {
				choke.Speed += speed
			}
		} else { 
			for _, speed := range peer.pod_up {
				choke.Speed += speed
			}
		}
		choke.Speed = choke.Speed/PONDERATION_TIME
		peers[addr] = choke
	}
	return peers
}

func (s *stats) GetSpeed(addr string) (speed int64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if peer, ok := s.peers[addr]; ok {
		if !s.bitfield.Completed() {
			for _, up := range peer.pod_up {
				speed += up
			}
			speed = speed/PONDERATION_TIME
		} else {
			for _, down := range peer.pod_down {
				speed += down
			}
			speed = speed/PONDERATION_TIME
		}
	}
	return speed
}

func (s *stats) GetGlobalStats() (int64, int64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.uploaded, s.downloaded
}

func NewStats(left, size int64, bitfield *bit_field.Bitfield, pieceLength int64) (st Stats) {
	s := new(stats)
	s.mutex = new(sync.Mutex)
	s.size = size
	s.peers = make(map[string] *PeerStat)
	s.pod_up, s.pod_down = make([]int64, PONDERATION_TIME), make([]int64, PONDERATION_TIME)
	s.bitfield = bitfield
	s.pieceLength = pieceLength
	go s.run()
	st = s
	return
}

func (s *stats) update(addr string, uploaded, downloaded int64) {
	if _, ok := s.peers[addr]; !ok {
		s.peers[addr] = new(PeerStat)
		s.peers[addr].pod_up = make([]int64, PONDERATION_TIME)
		s.peers[addr].pod_down = make([]int64, PONDERATION_TIME)
	}
	s.peers[addr].size_up += uploaded
	s.peers[addr].size_down += downloaded
}

func (s *stats) remove(addr string) {
	if _, ok := s.peers[addr]; ok {
		s.peers[addr] = nil, false
	}
}

func (s *stats) round() {
	//log.Println("Stats -> Start processing stats")
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
	var ratio float64
	if s.uploaded == 0 {
		ratio = 0
	} else if s.downloaded == 0 {
		if s.bitfield.Completed() {
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
	log.Println("Stats -> Downloading speed:", total_up/1000, "KB/s Uploading Speed:", total_down/1000, "KB/s Left:", (s.bitfield.Len() - s.bitfield.Count())*s.pieceLength/1000000, "MB Downloaded:", s.downloaded/1000000, "MB Uploaded:", s.uploaded/1000000, "MB Ratio:", fmt.Sprintf("%4.2f", ratio))
}

func (s *stats) run() {
	round := time.Tick(NS_PER_S)
	for {
		//log.Println("Stats -> Waiting for messages")
		select {
			case <- round:
				//log.Println("Stats -> Started processing stats")
				s.mutex.Lock()
				s.round()
				s.mutex.Unlock()
				//log.Println("Stats -> Finished processing stats")
		}
	}
}
