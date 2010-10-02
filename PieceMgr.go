// Management of pieces for a single torrent.
// Comunicates with PeerMgr (readers/writers) and Files
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package main

import(
	"encoding/binary"
	"os"
	"log"
	"bytes"
	"time"
	"math"
	)

type PieceMgrRequest struct {
	response chan *message
	our_addr string
	msg *message
	bitfield *Bitfield
}
	
type PieceMgr struct {
	requests chan *PieceMgrRequest
	peerMgr chan *message
	inStats chan string
	inFiles chan *FileMsg
	outStats chan *SpeedInfo
	pieceData *PieceData
	pieceLength, lastPieceLength, totalPieces, totalSize int64
	files FileStore
	bitfield *Bitfield
}

func NewPieceMgr(requests chan *PieceMgrRequest, peerMgr chan *message, inStats chan string, outStats chan *SpeedInfo, files FileStore, bitfield *Bitfield, pieceLength, lastPieceLength, totalPieces, totalSize int64, inFiles chan *FileMsg) (pieceMgr *PieceMgr, err os.Error){
	pieceMgr = new(PieceMgr)
	pieceMgr.files = files
	pieceMgr.pieceLength = pieceLength
	pieceMgr.lastPieceLength = lastPieceLength
	pieceMgr.totalPieces = totalPieces
	pieceMgr.bitfield = bitfield
	pieceMgr.requests = requests
	pieceMgr.pieceData = NewPieceData(bitfield, pieceLength, lastPieceLength)
	pieceMgr.totalSize = totalSize
	pieceMgr.peerMgr = peerMgr
	pieceMgr.outStats = outStats
	pieceMgr.inStats = inStats
	pieceMgr.inFiles = inFiles
	return
}

func (p *PieceMgr) Run() {
	cleanPieceData := time.Tick(CLEAN_REQUESTS*NS_PER_S)
	for {
		//log.Stderr("PieceMgr -> Waiting for messages")
		select {
			case msg := <- p.requests:
				//log.Stderr("PieceMgr -> New Request")
				switch msg.msg.msgId {
					case our_request:
						// We are requesing for pieces
						//log.Stderr("PieceMgr -> Self-request for a piece")
						p.ProcessRequest(msg)
						//log.Stderr("PieceMgr -> Finished processing self-request for a piece")
					case request:
						//log.Stderr("PieceMgr -> Peer requests piece")
						err := p.ProcessPeerRequest(msg)
						if err != nil {
							log.Stderr(err)
						}
						//log.Stderr("PieceMgr -> Finished processing peer requests for a piece")
					case piece:
						//log.Stderr("PieceMgr -> Peer sends a piece")
						err := p.ProcessPiece(msg.msg)
						if err != nil {
							log.Stderr(err)
						}
						//log.Stderr("PieceMgr -> Piece stored correctly")
					case exit:
						//log.Stderr("PieceMgr -> Peer exits")
						p.pieceData.RemoveAll(msg.msg.addr[0])
						//log.Stderr("PieceMgr -> Peer removed correctly")
				}
			case <- cleanPieceData:
				//log.Stderr("PieceMgr -> Cleaning piece data")
				p.pieceData.Clean()
				//log.Stderr("PieceMgr -> Finished cleaning piece data")
		}
		//log.Stderr("PieceMgr -> finished")
	}
}

func (p *PieceMgr) ProcessRequest(msg *PieceMgrRequest) {
	// Calculate the number of pieces we need to request to get 10s of pieces at our current speed.
	// Get speed from Stats
	p.inStats <- msg.our_addr
	speed := <- p.outStats
	// Calculate number of pieces to request to have 10s worth of pieces incoming
	requests := int64(DEFAULT_REQUESTS)
	if speed.upload != 0 {
		requests = int64(math.Ceil(float64(REQUESTS_LENGTH)/(float64(STANDARD_BLOCK_LENGTH)/float64(speed.upload))))
	}
	//log.Stderr("PieceMgr -> Requesting", requests, "from peer", msg.our_addr, "with speed:", speed.upload)
	for i := p.pieceData.NumPieces(msg.our_addr); i < MAX_REQUESTS && i < requests; i++ {
		//log.Stderr("PieceMgr -> Searching new piece")
		piece, block, err := p.pieceData.SearchPiece(msg.our_addr, msg.bitfield)
		//log.Stderr("PieceMgr -> Finished searching piece")
		if err != nil {
			//log.Stderr(err)
			return
		}
		//log.Stderr("PieceMgr -> Requesting piece", piece, ".", block)
		//log.Stderr("PieceMgr -> Sending piece trough channel")
		msg.response <- p.RequestBlock(piece, block)
		//log.Stderr("pieceMgr -> Finished sending piece trough channel")
	}
}

func (p *PieceMgr) RequestBlock(piece int64, block int) (msg *message) {
	msg = new(message)
	begin := int64(block) * int64(STANDARD_BLOCK_LENGTH)
	length := int64(STANDARD_BLOCK_LENGTH)
	if piece == p.totalPieces-1 {
		left := p.lastPieceLength - begin
		if left < length {
			length = left
		}
	}
	//log.Stderr("Requesting", piece, ".", block)
	msg.msgId = request
	msg.payLoad = make([]byte, 12)
	msg.length = uint32(1 + len(msg.payLoad))
	binary.BigEndian.PutUint32(msg.payLoad[0:4], uint32(piece))
	binary.BigEndian.PutUint32(msg.payLoad[4:8], uint32(begin))
	binary.BigEndian.PutUint32(msg.payLoad[8:12], uint32(length))
	return
}

func (p *PieceMgr) ProcessPiece(msg *message) (err os.Error){
	if msg.length < 9 {
		return os.NewError("Unexpected message length")
	}
	// Check which piece we have received
	index := binary.BigEndian.Uint32(msg.payLoad[0:4])
	begin := binary.BigEndian.Uint32(msg.payLoad[4:8])
	length := len(msg.payLoad) - 8
	if index >= uint32(p.bitfield.n) {
		return os.NewError("Piece out of range")
	}
	if p.bitfield.IsSet(int64(index)) {
		// We already have that piece, keep going
		return os.NewError("Piece already finished")
	}
	if int64(begin) >= p.pieceLength {
		return os.NewError("Begin out of range")
	}
	if int64(begin)+int64(length) > p.pieceLength {
		return os.NewError("Begin + length out of range")
	}
	if length > MAX_PIECE_LENGTH {
		return os.NewError("Block length too large")
	}
	finished, others := p.pieceData.Remove(msg.addr[0], int64(index), int64(begin)/STANDARD_BLOCK_LENGTH, true)
	if len(others) > 0 {
		// Send message to cancel request to other peers
		payLoad := make([]byte, 12)
		bytes.Add(payLoad, msg.payLoad[0:8])
		binary.BigEndian.PutUint32(payLoad[8:12], STANDARD_BLOCK_LENGTH)
		p.peerMgr <- &message{length: uint32(13), msgId: cancel, payLoad: payLoad, addr: others}
	}
	//log.Stderr("PieceMgr -> Received piece", index, ".", int(begin)/STANDARD_BLOCK_LENGTH)
	if !finished {
		return
	}
	fileMsg := new(FileMsg)
	fileMsg.Id = checkpiece
	fileMsg.Response = make(chan *FileMsg)
	fileMsg.Index = int64(index)
	fileMsg.Ok = false
	fileMsg.Err = nil
	p.inFiles <- fileMsg
	fileMsg = <- fileMsg.Response
	//ok, err := p.files.CheckPiece(int64(index))
	if !fileMsg.Ok {
		return os.NewError("Ignoring bad piece " + string(index) + " " + err.String())
	}
	// Mark piece as finished and delete it from activePieces
	p.bitfield.Set(int64(index))
	// Send have message to peerMgr to distribute it across peers
	p.peerMgr <- &message{length: uint32(5), msgId: have, payLoad: msg.payLoad[0:4]}
	log.Stderr("-------> Piece ", index, "finished")
	log.Stderr("Finished Pieces:", p.bitfield.Count(), "/", p.totalPieces)
	return
}

func (p *PieceMgr) ProcessPeerRequest(msg *PieceMgrRequest) (err os.Error) {
	if msg.msg.length < 9 {
		return os.NewError("Unexpected message length")
	}
	index := binary.BigEndian.Uint32(msg.msg.payLoad[0:4])
	if !p.bitfield.IsSet(int64(index)) {
		return os.NewError("Peer requests unfinished piece, ignoring request")
	}
	msg.msg.msgId = piece
	msg.response <- msg.msg
	//log.Stderr(message{length: length + uint32(9), msgId: piece, payLoad: buffer[0:length+8]})
	//log.Stderr("PieceMgr -> Peer", msg.msg.addr[0], "requests", index, ".", begin/STANDARD_BLOCK_LENGTH)
	return
}
