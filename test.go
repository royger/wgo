// Test functions
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package main

import(
	"log"
	"flag"
	"time"
	"runtime"
	"wgo/limiter"
	"wgo/files"
	"wgo/stats"
	"wgo/peers"
	"wgo/choke"
	"wgo/listener"
	"wgo/tracker"
	"strconv"
	"os"
	"rand"
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
	peerId := (CLIENT_ID + "-" + strconv.Itoa(os.Getpid()) + strconv.Itoa64(rand.Int63()))[0:20]
	log.Println(peerId)
	// Load torrent file
	torr, err := NewTorrent(*torrent)
	if err != nil {
		return
	}
	// Create File Store
	fs, size, err := files.NewFiles(&torr.Info, *folder)
	log.Println("Total size:", size)
	left, bitfield, err := fs.CheckPieces()
	if err != nil {
		log.Println(err)
		return
	}
	// BW Limiter
	limiter, err := limiter.NewLimiter(*up_limit, *down_limit)
	if err != nil {
		return
	}
	// Perform test of the tracker request
	// Initilize Stats
	s := stats.NewStats(left, size, bitfield, torr.Info.Piece_length)
	//go s.Run()
	// Initialize peerMgr
	lastPieceLength := size % torr.Info.Piece_length
	peerMgr, err := peers.NewPeerMgr(int64(bitfield.Len()), peerId, torr.Infohash, bitfield, s, fs, limiter, lastPieceLength)
	if err != nil {
		log.Println(err)
		return
	}
	if _, err = listener.NewListener(*ip, *listen_port, peerMgr); err != nil {
		panic(err)
	}
	//go peerMgr.Run()
	// Initialize ChokeMgr
	choke.NewChokeMgr(s, peerMgr)
	// Initialize pieceMgr
	pieceMgr, err := peers.NewPieceMgr(peerMgr, s, fs, bitfield, torr.Info.Piece_length, lastPieceLength, bitfield.Len(), size)
	peerMgr.SetPieceMgr(pieceMgr)
	tracker.NewTrackerMgr(torr.Announce_list, torr.Infohash, *listen_port, peerMgr, left, bitfield, torr.Info.Piece_length, peerId, s)
	for {
		log.Println("Active Peers:", peerMgr.ActivePeers(), "Incoming Peers:", peerMgr.IncomingPeers(), "Unused Peers:", peerMgr.UnusedPeers())
		log.Println("Done:", (bitfield.Count()*100)/bitfield.Len(), "%")
		log.Println("Bitfield:", bitfield.Bytes())
		time.Sleep(30*NS_PER_S)
	}
}
