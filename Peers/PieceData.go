// Magnagement of pieces requested by peers
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package peers

import(
	"time"
	"os"
	"rand"
	"wgo/bit_field"
	//"log"
	)
	
type PieceData struct {
	pieces map[int64]*Piece
	peers map[string]map[uint64]int64
	bitfield *bit_field.Bitfield
	pieceLength, lastPieceLength int64
}

type Piece struct {
	downloaderCount []int // -1 means piece is already downloaded
	pieceLength     int64
}

func NewPieceData(bitfield *bit_field.Bitfield, pieceLength, lastPieceLength int64) (p *PieceData) {
	p = new(PieceData)
	p.pieces = make(map[int64]*Piece, bitfield.Len())
	p.peers = make(map[string]map[uint64]int64, ACTIVE_PEERS + INCOMING_PEERS)
	p.bitfield = bitfield
	p.pieceLength = pieceLength
	p.lastPieceLength = lastPieceLength
	return
}

func NewPiece(pieceCount, pieceLength int64) (p *Piece) {
	p = new(Piece)
	p.pieceLength = pieceLength
	p.downloaderCount = make([]int, pieceCount)
	return
}

func (pd *PieceData) Add(addr string, pieceNum int64, blockNum int) {
	if _, ok := pd.pieces[pieceNum]; ok {
		pd.pieces[pieceNum].downloaderCount[blockNum]++
	} else {
		pieceLength :=  pd.pieceLength
		if pieceNum == pd.bitfield.Len()-1 {
			pieceLength = pd.lastPieceLength
		}
		pieceCount := (pieceLength + STANDARD_BLOCK_LENGTH - 1) / STANDARD_BLOCK_LENGTH
		pd.pieces[pieceNum] = NewPiece(pieceCount, pieceLength)
		pd.pieces[pieceNum].downloaderCount[blockNum]++
	}
	// Mark peer as downloading this piece
	ref := uint64(pieceNum) << 32 | uint64(blockNum)
	if _, ok := pd.peers[addr]; ok {
		pd.peers[addr][ref] = time.Seconds()
	} else {
		pd.peers[addr] = make(map[uint64]int64)
		pd.peers[addr][ref] = time.Seconds()
	}
}

func (pd *PieceData) CheckRequested(addr string, pieceNum int64, blockNum int) bool {
	ref := uint64(pieceNum) << 32 | uint64(blockNum)
	if peer, ok := pd.peers[addr]; ok {
		if _, ok := peer[ref]; ok {
			return true
		} else {
			return false
		}
	}
	return false
}

func (pd *PieceData) Remove(addr string, pieceNum, blockNum int64, finished bool) (pieceFinished bool, others []string) {
	if _, ok := pd.pieces[pieceNum]; ok {
		if finished {
			if pd.pieces[pieceNum].downloaderCount[blockNum] > 1 {
				others = pd.SearchPeers(pieceNum, blockNum, int64(pd.pieces[pieceNum].downloaderCount[blockNum] - 1), addr)
			}
			pd.pieces[pieceNum].downloaderCount[blockNum] = -1
		} else {
			if pd.pieces[pieceNum].downloaderCount[blockNum] > 0 {
				pd.pieces[pieceNum].downloaderCount[blockNum]--
			}
		}
		pieceFinished = true
		for _, block := range(pd.pieces[pieceNum].downloaderCount) {
			if block != -1 {
				pieceFinished = false
				break
			}
		}
		if pieceFinished {
			pd.pieces[pieceNum] = pd.pieces[pieceNum], false
		}
	}
	// Remove from peers
	if _, ok := pd.peers[addr]; ok {
		ref := uint64(pieceNum) << 32 | uint64(blockNum)
		if _, ok := pd.peers[addr][ref]; ok {
			pd.peers[addr][ref] = 0, false
		}
		if len(pd.peers[addr]) == 0 {
			pd.peers[addr] = nil, false
		}
	}
	return
}

func (pd *PieceData) RemoveAll(addr string) {
	//log.Println("PieceData -> Removing peer", addr)
	if peer, ok := pd.peers[addr]; ok {
		for ref, _ := range(peer) {
			pieceNum, blockNum := uint32(ref>>32), uint32(ref)
			pd.Remove(addr, int64(pieceNum), int64(blockNum), false)
		}
	}
	//log.Println("PieceData -> Finished removing peer", addr)
}

func (pd *PieceData) SearchPeers(rpiece, rblock, size int64, our_addr string) (others []string){
	others = make([]string, 0, size)
	for addr, _ := range(pd.peers) {
		if addr != our_addr {
			for ref, _ := range(pd.peers[addr]) {
				pieceNum, blockNum := uint32(ref>>32), uint32(ref)
				if int64(pieceNum) == rpiece && int64(blockNum) == rblock {
					// Add to return array
					//others[i] = addr
					//i++
					others = appendString(others, addr)
					// Remove from list
					pd.peers[addr][ref] = 0, false
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

func (pd *PieceData) SearchPiece(addr string, bitfield *bit_field.Bitfield) (rpiece int64, rblock int, err os.Error) {
	// Check if peer has some of the active pieces to finish them
	//log.Println("PieceData -> Searching for an already present piece")
	for k, piece := range (pd.pieces) {
		available := -1
		for block, downloads := range piece.downloaderCount {
			if downloads == 0 {
				available = block
				break
			}
		}
		if available != -1 && bitfield.IsSet(k) {
			// Send request piece k, block available
			pd.Add(addr, k, available)
			rpiece, rblock = k, available
			return
		}
	}
	//log.Println("PieceData -> No suitable piece found in active set")
	// Check what piece we can request
	totalPieces := pd.bitfield.Len()
	bytes := bitfield.Bytes()
	start := rand.Int63n(totalPieces)
	// Search fordward
	//log.Println("PieceData -> Searching fordwards")
	for piece := pd.bitfield.FindNextPiece(start, bytes); piece != -1 && piece < totalPieces; piece = pd.bitfield.FindNextPiece(piece+1, bytes) {
		//log.Println("PieceData -> Piece found, see if it's already in active piece set:", piece)
		if _, ok := pd.pieces[piece]; !ok {
			// Add new piece to set
			pd.Add(addr, piece, 0)
			rpiece, rblock = piece, 0
			return
		}
	}
	// Search backwards
	//log.Println("PieceData -> Searching backwards")
	for piece := pd.bitfield.FindNextPiece(0, bytes); piece != -1 && piece < start; piece = pd.bitfield.FindNextPiece(piece+1, bytes) {
		//log.Println("PieceData -> Piece found, see if it's already in active piece set:", piece)
		if _, ok := pd.pieces[piece]; !ok {
			// Add new piece to set
			pd.Add(addr, piece, 0)
			rpiece, rblock = piece, 0
			return
		}
	}
	// If all pieces are taken, double up on an active piece
	// if only 20% of pieces remaining
	//log.Println("PieceData -> Trying to enter endgame mode")
	if float64(pd.bitfield.Count())/float64(pd.bitfield.Len()) < 0.80 {
		err = os.NewError("No available block found")
		return
	}
	//log.Println("PieceData -> Doubling up on an active piece")
	first := true
	min := 0
	for k, piece := range (pd.pieces) {
		for block, downloads := range piece.downloaderCount {
			if bitfield.IsSet(k) && !pd.CheckRequested(addr, k, block) {
				if first && downloads != -1 {
					rpiece, rblock, min = k, block, downloads
					first = false
				}
				if downloads != -1 && downloads < min {
					rpiece, rblock, min = k, block, downloads
				}
			}
		}
	}
	if !first && min < MAX_PIECE_REQUESTS {
		pd.Add(addr, rpiece, rblock)
		return
	}
	err = os.NewError("No available block found")
	return
}

func (pd *PieceData) NumPieces(addr string) (n int64) {
	if peer, ok := pd.peers[addr]; ok {
		n = int64(len(peer))
	} else {
		n = 0
	}
	return
}

func (pd *PieceData) Clean() {
	actual := time.Seconds()
	for addr, peer := range(pd.peers) {
		for ref, time := range(peer) {
			if (actual - time) > CLEAN_REQUESTS {
				// Delete request
				pieceNum, blockNum := uint32(ref>>32), uint32(ref)
				pd.Remove(addr, int64(pieceNum), int64(blockNum), false)
			}
		}
	}
}

func appendString(slice []string, data string) []string {
	l := len(slice)
	if l + 1 > cap(slice) {  // reallocate
		// Allocate 10 more slots
		newSlice := make([]string, (l+10))
		// The copy function is predeclared and works for any slice type.
		copy(newSlice, slice)
		slice = newSlice
	}
	slice = slice[0:l+1]
	slice[l] = data
	return slice
}
