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
	"container/list"
	)

type PieceRequest struct {
	bitfield *Bitfield
	response chan message
	addr string
}

type Request struct {
	msg message
	response chan message
}
	
type PieceMgr struct {
	requests chan PieceRequest
	messages chan Request
	peerMgr chan message
	activePieces map[int64] *Piece
	peers map[string] *list.List
	pieceLength, lastPieceLength, totalPieces, totalSize int
	files FileStore
	bitfield *Bitfield
}

type Piece struct {
	downloaderCount []int // -1 means piece is already downloaded
	pieceLength     int
}

func NewPieceMgr(requests chan PieceRequest, messages chan Request, peerMgr chan message, files FileStore, bitfield *Bitfield, pieceLength, lastPieceLength, totalPieces, totalSize int) (pieceMgr *PieceMgr, err os.Error){
	pieceMgr = new(PieceMgr)
	pieceMgr.files = files
	pieceMgr.pieceLength = pieceLength
	pieceMgr.lastPieceLength = lastPieceLength
	pieceMgr.totalPieces = totalPieces
	pieceMgr.bitfield = bitfield
	pieceMgr.messages = messages
	pieceMgr.requests = requests
	pieceMgr.activePieces = make(map[int64]*Piece)
	pieceMgr.peers = make(map[string]*list.List)
	pieceMgr.totalSize = totalSize
	pieceMgr.peerMgr = peerMgr
	return
}

func (p *PieceMgr) Run() {
	for {
		select {
			case msg := <- p.requests:
				//log.Stderr("Received request")
				p.ProcessRequest(msg)
			case msg := <- p.messages:
				//log.Stderr("Received piece")
				switch msg.msg.msgId {
					case piece:
						err := p.ProcessPiece(msg.msg)
						if err != nil {
							log.Stderr(err)
						}
					case request:
						err := p.ProcessPeerRequest(msg)
						if err != nil {
							log.Stderr(err)
						}
					case exit:
						// Remove pieces requested to this peer
						err := p.RemoveAll(msg.msg.addr)
						if err != nil {
							log.Stderr(err)
						}
				}
		}
	}
}

func (p *PieceMgr) ProcessRequest(msg PieceRequest) {
	// Check if peer has some of the active pieces to finish it
	for k, piece := range (p.activePieces) {
		available := -1
		for offset, downloads := range piece.downloaderCount {
			if downloads == 0 {
				available = offset
				break
			}
		}
		if available != -1 && msg.bitfield.IsSet(int(k)) {
			// Send request piece k, block available
			piece.downloaderCount[available]++
			req := p.RequestBlock(int(k), available)
			msg.response <- req
			p.Add(msg.addr, req.payLoad[0:8])
			return
		}
	}
	// Check what piece we can request
	for i := 0; i < p.totalPieces; i++ {
		if (!p.bitfield.IsSet(i)) && msg.bitfield.IsSet(i) {
			if _, ok := p.activePieces[int64(i)]; !ok {
				// Add new piece to set
				pieceLength := 0
				if i == p.totalPieces-1 {
					pieceLength = p.lastPieceLength
				} else {
					pieceLength = p.pieceLength
				}
				pieceCount := (pieceLength + STANDARD_BLOCK_LENGTH - 1) / STANDARD_BLOCK_LENGTH
				p.activePieces[int64(i)] = NewPiece(pieceCount, pieceLength)
				p.activePieces[int64(i)].downloaderCount[0]++
				// Request 1st block of piece
				req := p.RequestBlock(i, 0)
				msg.response <- req
				p.Add(msg.addr, req.payLoad[0:8])
				return
			}
		}
	}
}

func (p *PieceMgr) RequestBlock(piece, block int) (msg message) {
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

func (p *PieceMgr) ProcessPiece(msg message) (err os.Error){
	if msg.length < 9 {
		return os.NewError("Unexpected message length")
	}
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
	/*t.RecordBlock(p, index, begin, uint32(length))
	err = t.RequestBlock(p)*/
	//block := begin / STANDARD_BLOCK_LENGTH
	//log.Stderr("Received piece", index, ".", block)
	p.Remove(msg.addr, msg.payLoad[0:8])
	//p.activePieces[int64(index)].downloaderCount[block] = -1
	for _, v := range(p.activePieces[int64(index)].downloaderCount) {
		if v != -1 {
			// If some of the parts are not finished
			return
		}
	}
	// Delete piece from activePieces set
	p.activePieces[int64(index)] = nil, false
	// Since it was the last part of the piece, mark it as finished
	ok, err := p.files.CheckPiece(int64(p.totalSize), int(index))
	if !ok || err != nil {
		return os.NewError("Ignoring bad piece " + string(index) + " " + err.String())
	}
	// Mark piece as finished and delete it from activePieces
	p.bitfield.Set(int(index))
	// Send have message to peerMgr to distribute it across peers
	p.peerMgr <- message{length: uint32(5), msgId: have, payLoad: msg.payLoad[0:4]}
	log.Stderr("-------> Piece ", index, "finished")
	log.Stderr("Finished Pieces:", p.bitfield.Count(), "/", p.totalPieces)
	return
}

func (p *PieceMgr) ProcessPeerRequest(msg Request) (err os.Error) {
	if msg.msg.length < 9 {
		return os.NewError("Unexpected message length")
	}
	index := binary.BigEndian.Uint32(msg.msg.payLoad[0:4])
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
	msg.response <- message{length: length + 8 + 1, msgId: piece, payLoad: buffer}
	return
}

func (p *PieceMgr) Remove(addr string, removePiece []byte) (err os.Error) {
	if peer, ok := p.peers[addr]; ok {
		for piece := peer.Front(); piece != nil; piece = piece.Next() {
			piecePosition := piece.Value.([]byte)
			if bytes.Equal(piecePosition, removePiece) {
				piecePos := binary.BigEndian.Uint32(piecePosition[0:4])
				blockPos := binary.BigEndian.Uint32(piecePosition[4:8])/STANDARD_BLOCK_LENGTH
				if pc, ok := p.activePieces[int64(piecePos)]; ok && pc.downloaderCount[blockPos] > 0 {
					pc.downloaderCount[blockPos]--
				}
				peer.Remove(piece)
			}
		}
		if peer.Len() == 0 {
			// List empty, remove
			p.peers[addr] = peer, false
		}
	}
	return
}

func (p *PieceMgr) RemoveAll(addr string) (err os.Error) {
	if peer, ok := p.peers[addr]; ok {
		for piece := peer.Front(); piece != nil; piece = piece.Next() {
			piecePosition := piece.Value.([]byte)
			piece := binary.BigEndian.Uint32(piecePosition[0:4])
			block := binary.BigEndian.Uint32(piecePosition[4:8])/STANDARD_BLOCK_LENGTH
			if pc, ok := p.activePieces[int64(piece)]; ok && pc.downloaderCount[block] > 0 {
				pc.downloaderCount[block]--
			}
		}
		// Remove peer queue
		p.peers[addr] = peer, false
	}
	return
}

func (p *PieceMgr) Add(addr string, addPiece []byte) (err os.Error) {
	if _, ok := p.peers[addr]; !ok {
		p.peers[addr] = list.New()
	}
	p.peers[addr].PushBack(addPiece)
	return
}

func NewPiece(pieceCount, pieceLength int) (p *Piece) {
	p = new(Piece)
	p.pieceLength = pieceLength
	p.downloaderCount = make([]int, pieceCount)
	return
}

