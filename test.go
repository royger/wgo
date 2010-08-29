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
var ip *string = flag.String("ip", "127.0.0.1", "local address to listen to")
var listen_port *string = flag.String("port", "0", "local port to listen to")
var procs *int = flag.Int("procs", 1, "number of processes")

func main() {
	flag.Parse()
	runtime.GOMAXPROCS(*procs)
	// Create channels for test
	outPeerMgr := make(chan peersList)
	inTracker := make(chan int)
	outStatus := make(chan *trackerStatusMsg)
	inStatus := make(chan *TrackerStatMsg)
	outListen := make(chan *net.Conn)
	// Load torrent file
	torr, err := NewTorrent(*torrent)
	if err != nil {
		return
	}
	// Create File Store
	fs, size, err := NewFileStore(&torr.Info, *folder)
	log.Stderr("Total size:", size)
	left, bitfield, err := fs.CheckPieces()
	if err != nil {
		log.Stderr(err)
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
	stats := make(chan *PeerStatMsg)
	s := NewStats(stats, inStatus, left, size)
	go s.Run()
	// Initialize peerMgr
	requests := make(chan *PieceMgrRequest)
	peerMgrChan := make(chan *message)
	peerMgr, err := NewPeerMgr(outPeerMgr, inTracker, int64(bitfield.Len()), t.peerId, torr.Infohash, requests, peerMgrChan, bitfield, stats, outListen)
	if err != nil {
		log.Stderr(err)
		return
	}
	go peerMgr.Run()
	// Initialize pieceMgr
	lastPieceLength := int(size % torr.Info.Piece_length)
	pieceMgr, err := NewPieceMgr(requests, peerMgrChan, fs, bitfield, torr.Info.Piece_length, int64(lastPieceLength), bitfield.Len(), size)
	go pieceMgr.Run()
	
	for {
		log.Stderr("Active Peers:", len(peerMgr.activePeers))
		log.Stderr("Inactive Peers:", len(peerMgr.inactivePeers))
		log.Stderr("Incoming Peers:", len(peerMgr.incomingPeers))
		log.Stderr("Unused Peers:", peerMgr.unusedPeers.Len())
		log.Stderr("Bitfield:", bitfield.Bytes())
		time.Sleep(30*NS_PER_S)
	}
}
