// Test functions
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package main

import(
	"log"
	"flag"
	"time"
	"runtime"
	)

var torrent *string = flag.String("torrent", "", "url or path to a torrent file")
var folder *string = flag.String("folder", ".", "local folder to save the download")
var procs *int = flag.Int("procs", 1, "number of processes")

func main() {
	flag.Parse()
	runtime.GOMAXPROCS(*procs)
	// Create channels for test
	outPeerMgr := make(chan peersList)
	outStatus := make(chan trackerStatusMsg)
	// Load torrent file
	torr, err := NewTorrent(*torrent)
	if err != nil {
		return
	}
	// Create File Store
	fs, size, err := NewFileStore(&torr.Info, *folder)
	log.Stderr("Total size:", size)
	_, _, bitfield, err := fs.CheckPieces()
	if err != nil {
		log.Stderr(err)
		return
	}
	// Perform test of the tracker request
	t := NewTracker(torr.Announce, torr.Infohash, "6666", outPeerMgr, outStatus)
	go t.Run()
	// Initilize Stats
	stats := make(chan *StatMsg)
	s := NewStats(stats)
	go s.Run()
	// Initialize peerMgr
	requests := make(chan *PieceMgrRequest)
	peerMgrChan := make(chan *message)
	peerMgr, err := NewPeerMgr(outPeerMgr, int64(bitfield.Len()), t.peerId, torr.Infohash, requests, peerMgrChan, bitfield, stats)
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
		log.Stderr("Unused Peers:", peerMgr.unusedPeers.Len())
		log.Stderr("Bitfield:", bitfield.Bytes())
		time.Sleep(30*NS_PER_S)
	}
}
