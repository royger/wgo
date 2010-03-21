// Test functions
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package main

import(
	"log"
	"flag"
	"os"
	"net"
	)

func request_test(announce, infohash, port string, outPeerMgr chan peersList, outStatus chan trackerStatusMsg) (err os.Error, peerId, addr string) {
	
	t := NewTracker(announce, infohash, port, outPeerMgr, outStatus)

	go t.Request()
	
	msgP := <- outPeerMgr
	for msg := msgP.peers.Front(); msg != nil; msg = msg.Next() {
		err = wire_test(infohash, t.peerId, msg.Value.(string))
		if err != nil {
			log.Stderr(err)
		}
	}

	msgS := <- outStatus
	log.Stderr("Complete:", msgS.Complete)
	log.Stderr("Incomplete:", msgS.Incomplete)
	
	peerId = t.peerId
	return
}

func filestore_test(info *InfoDict, fileDir string) (err os.Error) {
	fs, size, err := NewFileStore(info, fileDir)
	log.Stderr("Total size:", size)
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
		return os.NewError("WriteAt/ReadAt test failed")
	}
	return
}

func wire_test(infohash string, peerid string, addr string) (err os.Error) {
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
	// Perform test of the tracker request
	err, _, _ = request_test(torrent.Announce, torrent.InfoHash, "6654", outPeerMgr, outStatus)
	if err != nil {
		log.Stderr(err)
		return
	}
	err = filestore_test(&torrent.Info, *folder)
	if err != nil {
		log.Stderr(err)
		return
	}
	/*err = wire_test(torrent.InfoHash, peerId, addr)
	if err != nil {
		log.Stderr(err)
		return
	}*/
	
}
