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
	pieceData *PieceData
	pieceLength, lastPieceLength, totalPieces, totalSize int
	files FileStore
	bitfield *Bitfield
}

func NewPieceMgr(requests chan *PieceMgrRequest, peerMgr chan *message, files FileStore, bitfield *Bitfield, pieceLength, lastPieceLength, totalPieces, totalSize int) (pieceMgr *PieceMgr, err os.Error){
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
	return
}

func (p *PieceMgr) Run() {
	cleanPieceData := time.Tick(CLEAN_REQUESTS*NS_PER_S)
	for {
		log.Stderr("PieceMgr -> Waiting for messages")
		select {
			case msg := <- p.requests:
				log.Stderr("PieceMgr -> New Request")
				switch msg.msg.msgId {
					case our_request:
						// We are requesing for pieces
						log.Stderr("PieceMgr -> Self-request for a piece")
						p.ProcessRequest(msg)
						log.Stderr("PieceMgr -> Finished processing self-request for a piece")
					case request:
						log.Stderr("PieceMgr -> Peer requests piece")
						err := p.ProcessPeerRequest(msg)
						if err != nil {
							log.Stderr(err)
						}
						log.Stderr("PieceMgr -> Finished processing peer requests for a piece")
					case piece:
						log.Stderr("PieceMgr -> Peer sends a piece")
						err := p.ProcessPiece(msg.msg)
						if err != nil {
							log.Stderr(err)
						}
						log.Stderr("PieceMgr -> Piece stored correctly")
					case exit:
						log.Stderr("PieceMgr -> Peer exits")
						p.pieceData.RemoveAll(msg.msg.addr[0])
						log.Stderr("PieceMgr -> Peer removed correctly")
				}
			case <- cleanPieceData:
				log.Stderr("PieceMgr -> Cleaning piece data")
				p.pieceData.Clean()
				log.Stderr("PieceMgr -> Finished cleaning piece data")
		}
		//log.Stderr("PieceMgr -> finished")
	}
}

func (p *PieceMgr) ProcessRequest(msg *PieceMgrRequest) {
	for i := p.pieceData.NumPieces(msg.our_addr); i < MAX_REQUESTS; i++ {
		piece, block, err := p.pieceData.SearchPiece(msg.our_addr, msg.bitfield)
		if err != nil {
			log.Stderr(err)
		}
		//log.Stderr("PieceMgr -> Requesting piece", piece, ".", block)
		msg.response <- p.RequestBlock(piece, block)
	}
}

func (p *PieceMgr) RequestBlock(piece, block int) (msg *message) {
	msg = new(message)
	begin := block * STANDARD_BLOCK_LENGTH
	length := STANDARD_BLOCK_LENGTH
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
	if p.bitfield.IsSet(int(index)) {
		// We already have that piece, keep going
		return os.NewError("Piece already finished")
	}
	if int(begin) >= p.pieceLength {
		return os.NewError("Begin out of range")
	}
	if int(begin)+length > p.pieceLength {
		return os.NewError("Begin + length out of range")
	}
	if length > MAX_PIECE_LENGTH {
		return os.NewError("Block length too large")
	}
	globalOffset := int(index)*p.pieceLength + int(begin)
	// Write piece to FS
	_, err = p.files.WriteAt(msg.payLoad[8:], int64(globalOffset))
	if err != nil {
		return err
	}
	finished, others := p.pieceData.Remove(msg.addr[0], int(index), int(begin)/STANDARD_BLOCK_LENGTH, true)
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
	ok, err := p.files.CheckPiece(int64(p.totalSize), int(index))
	if !ok || err != nil {
		return os.NewError("Ignoring bad piece " + string(index) + " " + err.String())
	}
	// Mark piece as finished and delete it from activePieces
	p.bitfield.Set(int(index))
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
	if !p.bitfield.IsSet(int(index)) {
		return os.NewError("Peer requests unfinished piece, ignoring request")
	}
	begin := binary.BigEndian.Uint32(msg.msg.payLoad[4:8])
	length := binary.BigEndian.Uint32(msg.msg.payLoad[8:12])
	globalOffset := int(index)*p.pieceLength + int(begin)
	buffer := make([]byte, length + 8)
	_, err = p.files.ReadAt(buffer[8:], int64(globalOffset))
	if err != nil {
		return
	}
	bytes.Add(buffer[0:], msg.msg.payLoad[0:4])
	bytes.Add(buffer[4:], msg.msg.payLoad[4:8])
	msg.response <- &message{length: length + 8 + 1, msgId: piece, payLoad: buffer}
	//log.Stderr("PieceMgr -> Peer", msg.msg.addr[0], "requests", index, ".", begin/STANDARD_BLOCK_LENGTH)
	return
}
