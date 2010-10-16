// Test functions
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package main

import(
	"log"
	"flag"
	"time"
	"net"
	"runtime"
	)

var torrent *string = flag.String("torrent", "", "url or path to a torrent file")
var folder *string = flag.String("folder", ".", "local folder to save the download")
var ip *string = flag.String("ip", "", "local address to listen to")
var listen_port *string = flag.String("port", "0", "local port to listen to")
var procs *int = flag.Int("procs", 1, "number of processes")
var up_limit *int = flag.Int("up_limit", 0, "Upload limit in KB/s")
var down_limit *int = flag.Int("down_limit", 0, "Download limit in KB/s")


func main() {
	flag.Parse()
	runtime.GOMAXPROCS(*procs)
	// Create channels for test
	outPeerMgr := make(chan peersList)
	inTracker := make(chan int)
	outStatus := make(chan *Status)
	inStatus := make(chan *Status)
	outListen := make(chan net.Conn)
	inPeerMgr := make(chan chan map[string]*Peer)
	outChokeMgr := make(chan chan map[string]*Status)
	outPieceMgrInStats := make(chan string)
	outStatsInPieceMgr := make(chan *Status)
	outPeerInFiles := make(chan *FileMsg)
	// Load torrent file
	torr, err := NewTorrent(*torrent)
	if err != nil {
		return
	}
	// Create File Store
	fs, size, err := NewFileStore(&torr.Info, *folder, outPeerInFiles)
	log.Println("Total size:", size)
	left, bitfield, err := fs.CheckPieces()
	if err != nil {
		log.Println(err)
		return
	}
	go fs.Run()
	// BW Limiter
	limiter, err := NewLimiter(*up_limit, *down_limit)
	if err != nil {
		return
	}
	l, err := NewListener(*ip, *listen_port, outListen)
	if err != nil {
		panic(err)
	}
	go l.Run()
	// Perform test of the tracker request
	t := NewTracker(torr.Announce, torr.Infohash, *listen_port, outPeerMgr, inTracker, outStatus, inStatus, left)
	go t.Run()
	// Initilize Stats
	stats := make(chan *Status)
	s := NewStats(stats, inStatus, outChokeMgr, outPieceMgrInStats, outStatsInPieceMgr, left, size)
	go s.Run()
	// Initialize peerMgr
	requests := make(chan *PieceMgrRequest)
	peerMgrChan := make(chan *message)
	peerMgr, err := NewPeerMgr(outPeerMgr, inTracker, int64(bitfield.Len()), t.peerId, torr.Infohash, requests, peerMgrChan, bitfield, stats, outListen, inPeerMgr, outPeerInFiles, limiter.upload, limiter.download)
	if err != nil {
		log.Println(err)
		return
	}
	go peerMgr.Run()
	// Initialize ChokeMgr
	chokeMgr, _ := NewChokeMgr(outChokeMgr, inPeerMgr)
	go chokeMgr.Run()
	// Initialize pieceMgr
	lastPieceLength := int(size % torr.Info.Piece_length)
	pieceMgr, err := NewPieceMgr(requests, peerMgrChan, outPieceMgrInStats, outStatsInPieceMgr, fs, bitfield, torr.Info.Piece_length, int64(lastPieceLength), bitfield.Len(), size, outPeerInFiles)
	go pieceMgr.Run()
	
	for {
		log.Println("Active Peers:", len(peerMgr.activePeers))
		//log.Println("Inactive Peers:", len(peerMgr.inactivePeers))
		log.Println("Incoming Peers:", len(peerMgr.incomingPeers))
		log.Println("Unused Peers:", peerMgr.unusedPeers.Len())
		log.Println("Bitfield:", bitfield.Bytes())
		time.Sleep(30*NS_PER_S)
	}
}
