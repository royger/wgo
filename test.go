// Test functions

package main

import(
	"log"
	"flag"
	"os"
	)

func request_test(announce, infohash, port string, outPeerMgr chan peersList, outStatus chan trackerStatusMsg) (err os.Error) {
	
	t := NewTracker(announce, infohash, port, outPeerMgr, outStatus)

	go t.Request()
	
	msgP := <- outPeerMgr
	for msg := msgP.peers.Front(); msg != nil; msg = msg.Next() {
		log.Stderr(msg.Value)
	}

	msgS := <- outStatus
	log.Stderr("Complete:", msgS.Complete)
	log.Stderr("Incomplete:", msgS.Incomplete)
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
	err = request_test(torrent.Announce, torrent.InfoHash, "6654", outPeerMgr, outStatus)
	if err != nil {
		log.Stderr(err)
		return
	}
	err = filestore_test(&torrent.Info, *folder)
	if err != nil {
		log.Stderr(err)
		return
	}
}
