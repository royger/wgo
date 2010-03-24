// Test functions
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package main

import(
	"log"
	"flag"
	"os"
	"net"
	"time"
	)

func request_test(announce, infohash, port string, outPeerMgr chan peersList, outStatus chan trackerStatusMsg) (err os.Error, peerId, addr string) {
	
	t := NewTracker(announce, infohash, port, outPeerMgr, outStatus)

	go t.Run()
	
	peerId = t.peerId
	return
}

func filestore_test(torrent *Torrent, fileDir string) (numPieces int64, err os.Error) {
	fs, size, err := NewFileStore(&torrent.Info, fileDir)
	log.Stderr("Total size:", size)
	good, bad, goodBits, err := fs.CheckPieces(size, torrent)
	numPieces = good + bad
	if err != nil {
		return
	}
	log.Stderr("Good:", good, "Bad:", bad, "Bitfield:", goodBits)
	p := make([]byte, 512)
	b := make([]byte, 512)
	p[0] = 1
	_, err = fs.WriteAt(p, 0)
	if err != nil {
		return
	}
	_, err = fs.ReadAt(b, 0)
	if err != nil {
		return
	}
	if b[0] != p[0] {
		err = os.NewError("WriteAt/ReadAt test failed")
		return
	}
	return
}

func wire_test(infohash string, peerid string, addr string, numPieces int64) (err os.Error) {
	addrTCP, err := net.ResolveTCPAddr(addr)
	if err != nil {
		return
	}
	log.Stderr("Connecting to", addr)
	conn, err := net.DialTCP("tcp4", nil, addrTCP)
	defer conn.Close()
	if err != nil {
		return
	}
	w := NewWire(infohash, peerid, conn)
	log.Stderr("Sending Handshake")
	remotepeerid, err := w.Handshake()
	if err != nil {
		return
	}
	log.Stderr("Connected to peer with ID", remotepeerid)
	msg, err := w.ReadMsg()
	if err != nil {
		log.Stderr(err)
		return
	}
	if msg.msgId == 5 {
		bitfield, err := NewBitfieldFromBytes(int(numPieces), msg.payLoad)
		if err != nil {
			return
		}
		log.Stderr(bitfield)
	}
	log.Stderr(msg, "len payload:", len(msg.payLoad))
	log.Stderrf("%x", msg.payLoad)
	msg.length = 0
	log.Stderr("Sending keep-alive msg")
	err = w.WriteMsg(message{})
	if err != nil {
		log.Stderr(err)
		return
	}
	log.Stderr("Waiting for messages")
	msg, err = w.ReadMsg()
	if err != nil {
		log.Stderr(err)
		return
	}
	log.Stderr(msg)
	return
}

var torrent *string = flag.String("torrent", "", "url or path to a torrent file")
var folder *string = flag.String("folder", "", "local folder to save the download")

func main() {
	flag.Parse()
	
	// Create channels for test
	outPeerMgr := make(chan peersList)
	outStatus := make(chan trackerStatusMsg)
	// Load torrent file
	torrent, err := NewTorrent(*torrent)
	if err != nil {
		return
	}
	// File Store test
	numPieces, err := filestore_test(torrent, *folder)
	if err != nil {
		log.Stderr(err)
		return
	}
	// Perform test of the tracker request
	err, peerId, _ := request_test(torrent.Announce, torrent.InfoHash, "6654", outPeerMgr, outStatus)
	if err != nil {
		log.Stderr(err)
		return
	}
	
	peerMgr, err := NewPeerMgr(outPeerMgr, numPieces, peerId, torrent.InfoHash)
	if err != nil {
		log.Stderr(err)
		return
	}
	go peerMgr.Run()
	
	//msgS := <- outStatus
	/*log.Stderr("Complete:", msgS.Complete)
	log.Stderr("Incomplete:", msgS.Incomplete)
	log.Stderr("Num pieces:", numPieces)*/
	for {
		log.Stderr("Active Peers:", len(peerMgr.activePeers))
		log.Stderr("Inactive Peers:", len(peerMgr.inactivePeers))
		log.Stderr("Unused Peers:", peerMgr.unusedPeers.Len())
		time.Sleep(30*NS_PER_S)
	}
	/*
	msgP := <- outPeerMgr
	msgS := <- outStatus
	log.Stderr("Complete:", msgS.Complete)
	log.Stderr("Incomplete:", msgS.Incomplete)
	log.Stderr("Num pieces:", numPieces)
	
	incoming := make(chan message)
	outgoing := make(chan message)
	for msg := msgP.peers.Front(); msg != nil; msg = msg.Next() {
		//err = wire_test(torrent.InfoHash, peerId, msg.Value.(string), numPieces)
		p, err := NewPeer(msg.Value.(string), torrent.InfoHash, peerId, incoming, outgoing)
		if err != nil {
			log.Stderr(err)
		}
		go p.PeerWriter()
	}
	
	for msg := range outgoing {
		log.Stderr(msg)
	}
	// Test fileStore
	/*err = wire_test(torrent.InfoHash, peerId, addr)
	if err != nil {
		log.Stderr(err)
		return
	}*/
	
}
