// Test functions
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package main

import(
	"log"
	"flag"
	"time"
	)

var torrent *string = flag.String("torrent", "", "url or path to a torrent file")
var folder *string = flag.String("folder", "", "local folder to save the download")

func main() {
	flag.Parse()
	
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
	_, _, bitfield, err := fs.CheckPieces(size)
	if err != nil {
		log.Stderr(err)
		return
	}
	// Perform test of the tracker request
	t := NewTracker(torr.Announce, torr.InfoHash, "6666", outPeerMgr, outStatus)
	go t.Run()
	// Initialize peerMgr
	requests := make(chan *PieceMgrRequest)
	peerMgrChan := make(chan *message)
	peerMgr, err := NewPeerMgr(outPeerMgr, int64(bitfield.Len()), t.peerId, torr.InfoHash, requests, peerMgrChan, bitfield)
	if err != nil {
		log.Stderr(err)
		return
	}
	go peerMgr.Run()
	// Initialize pieceMgr
	lastPieceLength := int(size % torr.Info.PieceLength)
	pieceMgr, err := NewPieceMgr(requests, peerMgrChan, fs, bitfield, int(torr.Info.PieceLength), lastPieceLength, int(bitfield.Len()), int(size))
	go pieceMgr.Run()
	for {
		log.Stderr("Active Peers:", len(peerMgr.activePeers))
		log.Stderr("Inactive Peers:", len(peerMgr.inactivePeers))
		log.Stderr("Unused Peers:", peerMgr.unusedPeers.Len())
		log.Stderr("Bitfield:", bitfield.Bytes())
		time.Sleep(30*NS_PER_S)
	}
}
