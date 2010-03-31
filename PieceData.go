// Magnagement of pieces requested by peers
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package main

import(
	"time"
	"os"
	"rand"
	)
	
type PieceData struct {
	pieces map[int64]*Piece
	peers map[string]map[uint64]int64
	bitfield *Bitfield
	pieceLength, lastPieceLength int
}

type Piece struct {
	downloaderCount []int // -1 means piece is already downloaded
	pieceLength     int
}

func NewPieceData(bitfield *Bitfield, pieceLength, lastPieceLength int) (p *PieceData) {
	p = new(PieceData)
	p.pieces = make(map[int64]*Piece)
	p.peers = make(map[string]map[uint64]int64)
	p.bitfield = bitfield
	p.pieceLength = pieceLength
	p.lastPieceLength = lastPieceLength
	return
}

func NewPiece(pieceCount, pieceLength int) (p *Piece) {
	p = new(Piece)
	p.pieceLength = pieceLength
	p.downloaderCount = make([]int, pieceCount)
	return
}


func (pd *PieceData) Add(addr string, pieceNum int, blockNum int) {
	if piece, ok := pd.pieces[int64(pieceNum)]; ok {
		piece.downloaderCount[blockNum]++
	} else {
		pieceLength :=  pd.pieceLength
		if pieceNum == pd.bitfield.Len()-1 {
			pieceLength = pd.lastPieceLength
		}
		pieceCount := (pieceLength + STANDARD_BLOCK_LENGTH - 1) / STANDARD_BLOCK_LENGTH
		pd.pieces[int64(pieceNum)] = NewPiece(pieceCount, pieceLength)
		pd.pieces[int64(pieceNum)].downloaderCount[blockNum]++
	}
	// Mark peer as downloading this piece
	ref := uint64(pieceNum) << 32 | uint64(blockNum)
	if peer, ok := pd.peers[addr]; ok {
		peer[ref] = time.Seconds()
	} else {
		pd.peers[addr] = make(map[uint64]int64)
		pd.peers[addr][ref] = time.Seconds()
	}
}

func (pd *PieceData) Remove(addr string, pieceNum int, blockNum int, finished bool) (pieceFinished bool, others []string) {
	if piece, ok := pd.pieces[int64(pieceNum)]; ok {
		if finished {
			if piece.downloaderCount[blockNum] > 1 {
				others = pd.SearchPeers(pieceNum, blockNum, piece.downloaderCount[blockNum] - 1, addr)
			}
			piece.downloaderCount[blockNum] = -1
		} else {
			if piece.downloaderCount[blockNum] > 0 {
				piece.downloaderCount[blockNum]--
			}
		}
		pieceFinished = true
		for _, block := range(piece.downloaderCount) {
			if block != -1 {
				pieceFinished = false
				break
			}
		}
		if pieceFinished {
			pd.pieces[int64(pieceNum)] = piece, false
		}
	}
	// Remove from peers
	if peer, ok := pd.peers[addr]; ok {
		ref := uint64(pieceNum) << 32 | uint64(blockNum)
		if _, ok := peer[ref]; ok {
			peer[ref] = 0, false
		}
		if len(peer) == 0 {
			pd.peers[addr] = nil, false
		}
	}
	return
}

func (pd *PieceData) RemoveAll(addr string) {
	if peer, ok := pd.peers[addr]; ok {
		for ref, _ := range(peer) {
			pieceNum, blockNum := uint32(ref>>32), uint32(ref)
			pd.Remove(addr, int(pieceNum), int(blockNum), false)
		}
	}
}

func (pd *PieceData) SearchPeers(rpiece, rblock, size int, our_addr string) (others []string){
	others = make([]string, size)
	i := 0
	for addr, peer := range(pd.peers) {
		if addr != our_addr {
			for ref, _ := range(peer) {
				pieceNum, blockNum := uint32(ref>>32), uint32(ref)
				if int(pieceNum) == rpiece && int(blockNum) == rblock {
					// Add to return array
					others[i] = addr
					i++
					// Remove from list
					peer[ref] = 0, false
					// If peer list is empty, remove peer
					if len(pd.peers[addr]) == 0 {
						pd.peers[addr] = nil, false
					}
				}
			}
		}
	}
	return
}

func (pd *PieceData) SearchPiece(addr string, bitfield *Bitfield) (rpiece, rblock int, err os.Error) {
	// Check if peer has some of the active pieces to finish it
	for k, piece := range (pd.pieces) {
		available := -1
		for block, downloads := range piece.downloaderCount {
			if downloads == 0 {
				available = block
				break
			}
		}
		if available != -1 && bitfield.IsSet(int(k)) {
			// Send request piece k, block available
			pd.Add(addr, int(k), available)
			rpiece, rblock = int(k), available
			return
		}
	}
	// Check what piece we can request
	totalPieces := pd.bitfield.Len()
	start := rand.Intn(totalPieces)
	for i := start; i < totalPieces; i++ {
		if !pd.bitfield.IsSet(i) && bitfield.IsSet(i) {
			if _, ok := pd.pieces[int64(i)]; !ok {
				// Add new piece to set
				pd.Add(addr, i, 0)
				rpiece, rblock = i, 0
				return
			}
		}
	}
	for i:= 0; i < start; i++ {
		if !pd.bitfield.IsSet(i) && bitfield.IsSet(i) {
			if _, ok := pd.pieces[int64(i)]; !ok {
				// Add new piece to set
				pd.Add(addr, i, 0)
				rpiece, rblock = i, 0
				return
			}
		}
	}
	// If all pieces are taken, double up on an active piece
	first := true
	min := 0
	for k, piece := range (pd.pieces) {
		for block, downloads := range piece.downloaderCount {
			if bitfield.IsSet(int(k)) {
				if first && downloads != -1 {
					rpiece, rblock, min = int(k), block, downloads
					first = false
				}
				if downloads != -1 && downloads < min {
					rpiece, rblock, min = int(k), block, downloads
				}
			}
		}
	}
	if !first {
		pd.Add(addr, rpiece, rblock)
		return
	}
	err = os.NewError("No available block found")
	return
}

func (pd *PieceData) NumPieces(addr string) (n int) {
	if peer, ok := pd.peers[addr]; ok {
		n = len(peer)
	} else {
		n = 0
	}
	return
}

func (pd *PieceData) Clean() {
	actual := time.Seconds()
	for addr, peer := range(pd.peers) {
		for ref, time := range(peer) {
			if actual - time > CLEAN_REQUESTS {
				// Delete request
				pieceNum, blockNum := uint32(ref>>32), uint32(ref)
				pd.Remove(addr, int(pieceNum), int(blockNum), false)
			}
		}
	}
}
