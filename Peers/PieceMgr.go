// Management of pieces for a single torrent.
// Comunicates with PeerMgr (readers/writers) and Files
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package peers

import(
	"os"
	"log"
	"time"
	"math"
	"wgo/bit_field"
	"wgo/files"
	"wgo/stats"
	"sync"
	)

const(
	ACTIVE_PEERS = 45
	INCOMING_PEERS = 10
	STANDARD_BLOCK_LENGTH = 16 * 1024
	MAX_PIECE_REQUESTS = 4
	CLEAN_REQUESTS = 120
	DEFAULT_REQUESTS = 20
	REQUESTS_LENGTH = 10 // time of requests to ask to a peer (10s of pieces)
	NS_PER_S = 1000000000
	MAX_REQUESTS = 2048
	MAX_PIECE_LENGTH = 128*1024
)
	
type pieceMgr struct {
	mutex *sync.Mutex
	peerMgr PeerMgr
	stats stats.Stats
	pieceData *PieceData
	pieceLength, lastPieceLength, totalPieces, totalSize int64
	files files.Files
	bitfield *bit_field.Bitfield
}

type PieceMgr interface {
	Request(addr string, peer *Peer, bitfield *bit_field.Bitfield)
	SavePiece(addr string, index, begin, length int64) (os.Error)
	PeerExit(addr string)
}

func (p *pieceMgr) Request(addr string, peer *Peer, bitfield *bit_field.Bitfield) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	speed := p.stats.GetSpeed(addr)
	// Calculate number of pieces to request to have 10s worth of pieces incoming
	requests := int64(DEFAULT_REQUESTS)
	if speed != 0 {
		requests = int64(math.Ceil(float64(REQUESTS_LENGTH)/(float64(STANDARD_BLOCK_LENGTH)/float64(speed))))
	}
	//log.Println("PieceMgr -> Requesting", requests, "from peer", msg.our_addr, "with speed:", speed.upload)
	for i := p.pieceData.NumPieces(addr); i < MAX_REQUESTS && i < requests; i++ {
		//log.Println("PieceMgr -> Searching new piece")
		piece, block, err := p.pieceData.SearchPiece(addr, bitfield)
		//log.Println("PieceMgr -> Finished searching piece")
		if err != nil {
			//log.Println(err)
			return
		}
		// Add a method to peer to do enqueue the request
		peer.Request(piece, block)
		//log.Println("pieceMgr -> Finished sending piece trough channel")
	}
}

func (p *pieceMgr) SavePiece(addr string, index, begin, length int64) (os.Error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if length < 9 {
		return os.NewError("Unexpected message length")
	}
	if index >= p.bitfield.Len() {
		return os.NewError("Piece out of range")
	}
	if p.bitfield.IsSet(index) {
		// We already have that piece, keep going
		return os.NewError("Piece already finished")
	}
	if begin >= p.pieceLength {
		return os.NewError("Begin out of range")
	}
	if begin+length > p.pieceLength {
		return os.NewError("Begin + length out of range")
	}
	if length > MAX_PIECE_LENGTH {
		return os.NewError("Block length too large")
	}
	finished, others := p.pieceData.Remove(addr, index, begin/STANDARD_BLOCK_LENGTH, true)
	if len(others) > 0 {
		// Send message to cancel request to other peers
		p.peerMgr.SendCancel(others, index, begin, STANDARD_BLOCK_LENGTH)
	}
	if !finished {
		return nil
	}
	if err := p.files.CheckPiece(index); err != nil {
		return os.NewError("Ignoring bad piece " + string(index))
	}
	// Mark piece as finished and delete it from activePieces
	p.bitfield.Set(index)
	// Send have message to peerMgr to distribute it across peers
	p.peerMgr.SendHave(index)
	log.Println("-------> Piece ", index, "finished")
	log.Println("Finished Pieces:", p.bitfield.Count(), "/", p.totalPieces)
	return nil
}

func (p *pieceMgr) PeerExit(addr string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.pieceData.RemoveAll(addr)
}

func NewPieceMgr(peerMgr PeerMgr, st stats.Stats, fl files.Files, bitfield *bit_field.Bitfield, pieceLength, lastPieceLength, totalPieces, totalSize int64) (p PieceMgr, err os.Error){
	pieceMgr := new(pieceMgr)
	pieceMgr.mutex = new(sync.Mutex)
	pieceMgr.files = fl
	pieceMgr.pieceLength = pieceLength
	pieceMgr.lastPieceLength = lastPieceLength
	pieceMgr.totalPieces = totalPieces
	pieceMgr.bitfield = bitfield
	pieceMgr.pieceData = NewPieceData(bitfield, pieceLength, lastPieceLength)
	pieceMgr.totalSize = totalSize
	pieceMgr.peerMgr = peerMgr
	pieceMgr.stats = st
	pieceMgr.files = fl
	p = pieceMgr
	go pieceMgr.Run()
	return
}

func (p *pieceMgr) Run() {
	cleanPieceData := time.Tick(CLEAN_REQUESTS*NS_PER_S)
	for {
		//log.Println("PieceMgr -> Waiting for messages")
		select {
			case <- cleanPieceData:
				p.mutex.Lock()
				//log.Println("PieceMgr -> Cleaning piece data")
				p.pieceData.Clean()
				//log.Println("PieceMgr -> Finished cleaning piece data")
				p.mutex.Unlock()
		}
		//log.Println("PieceMgr -> finished")
	}
}
