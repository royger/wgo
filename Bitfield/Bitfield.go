package bit_field

import(
	"os"
	"sync"
	)

// As defined by the bittorrent protocol, this bitset is big-endian, such that
// the high bit of the first byte is block 0

type Bitfield struct {
	b        []byte
	n        int64
	done     int64
	endIndex int64
	endMask  byte // Which bits of the last byte are valid
	mutex *sync.RWMutex
}

func NewBitfield(n int64) (bitfield *Bitfield) {
	endIndex, endOffset := n>>3, n&7
	endMask := ^byte(255 >> byte(endOffset))
	if endOffset == 0 {
		endIndex = -1
	}
	bitfield = &Bitfield{make([]byte, (n+7)>>3), n, 0, endIndex, endMask, new(sync.RWMutex)}
	return
}

// Creates a new bitset from a given byte stream.

func NewBitfieldFromBytes(n int64, data []byte) (bitfield *Bitfield, err os.Error) {
	bitfield = NewBitfield(n)
	if len(bitfield.b) != len(data) {
		return bitfield, os.NewError("Invalid length of bitfield")
	}
	copy(bitfield.b, data)
	if bitfield.endIndex >= 0 && bitfield.b[bitfield.endIndex]&(^bitfield.endMask) != 0 {
		return bitfield, os.NewError("Invalid bitfield")
	}
	for i := int64(0); i < n; i++ {
		if bitfield.IsSet(i) {
			bitfield.done++
		}
	}
	return
}

func (b *Bitfield) Set(index int64) {
	b.mutex.Lock()
	//log.Println("Bitfield Set")
	defer b.mutex.Unlock()
	if index < 0 || index >= b.n {
		panic("Index out of range.")
	}
	b.b[index>>3] |= byte(128 >> byte(index&7))
	b.done++
	//log.Println("Bitfield Set Exit")
	return
}

func (b *Bitfield) IsSet(index int64) bool {
	//log.Println("Trying Bitfield IsSet")
	b.mutex.RLock()
	//log.Println("Bitfield IsSet")
	defer b.mutex.RUnlock()
	if index < 0 || index >= b.n {
		panic("Index out of range.")
	}
	//log.Println("Bitfield IsSet Exit")
	//result = (b.b[index>>3] & byte(128>>byte(index&7))) != 0
	return (b.b[index>>3] & byte(128>>byte(index&7))) != 0
}

func (b *Bitfield) Bytes() []byte {
	b.mutex.RLock()
	//log.Println("Bitfield Bytes")
	defer b.mutex.RUnlock()
	//bitfield = b.b
	//log.Println("Bitfield Bytes Exit")
	bitfield := make([]byte, len(b.b))
	copy(bitfield, b.b)
	
	return bitfield
}

func (b *Bitfield) Len() int64 {
	b.mutex.RLock()
	//log.Println("Bitfield Len")
	defer b.mutex.RUnlock()
	//log.Println("Bitfield Len Exit")
	return b.n
}

func (b *Bitfield) HasMorePieces(p []byte) bool {
	b.mutex.RLock()
	//log.Println("Bitfield HasMorePieces")
	defer b.mutex.RUnlock()
	for i := 0; i < len(b.b); i++ {
		if (p[i] & ^b.b[i]) > 0 {
			//log.Println("Bitfield HasMorePieces Exit")
			return true
		}
	}
	//log.Println("Bitfield HasMorePieces Exit")
	return false
}

/*func (b *Bitfield) FindNextPiece(startp, startb, num *int, p []byte) int64 {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	for ; *num < len(b.b); *startp, *num = (*startp+1)%len(b.b), *num+1 {
		if (p[*startp] & ^b.b[*startp]) > 0 {
			piece := p[*startp] & ^b.b[*startp]
			for *startb++; *startb < 8; *startb++ {
				if (piece & byte(1<<byte(*startb))) > 0 {
					return int64((*startp)*8+*startb)
				}
			}
			*startb = 0
		}
	}
	return -1
}*/

func (b *Bitfield) FindNextPiece(start int64, p []byte) int64 {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	var piece byte
	for i := int(start/8); i < len(b.b); i++ {
		piece = p[i] & ^b.b[i]
		if piece > 0 {
			for j := 7 - int(start%8); j >= 0; j-- {
				if (piece & byte(1<<byte(j))) > 0 {
					return int64(i*8+(7-j))
				}
			}
		}
		start = 0
	}
	return -1
}

func (b *Bitfield) Count() int64 {
	//log.Println("Trying Bitfield Count")
	b.mutex.RLock()
	//log.Println("Bitfield Count")
	defer b.mutex.RUnlock()
	return b.done
}

func (b *Bitfield) Completed() bool {
	//log.Println("Trying Bitfield Completed")
	b.mutex.RLock()
	//log.Println("Bitfield Completed")
	defer b.mutex.RUnlock()
	if b.done == b.n {
		//log.Println("Bitfield Completed Exit")
		return true
	}
	//log.Println("Bitfield Completed Exit")
	return false
}

// TODO: Make this fast
/*func (b *Bitfield) FindNextSet(index int64) int64 {
	b.mutex.RLock()
	//log.Println("Bitfield FindNextSet")
	defer b.mutex.RUnlock()
	for i := index; i < b.n; i++ {
		if (b.b[i>>3] & byte(128>>byte(i&7))) != 0 {
			//log.Println("Bitfield FindNextSet Exit")
			return i
		}
	}
	//log.Println("Bitfield FindNExtSet Exit")
	return -1
}*/

// TODO: Make this fast
/*func (b *Bitfield) FindNextClear(index int64) int64 {
	b.mutex.RLock()
	//log.Println("Bitfield FidnNextClear")
	defer b.mutex.RUnlock()
	for i := index; i < b.n; i++ {
		if (b.b[i>>3] & byte(128>>byte(i&7))) == 0 {
			//log.Println("Bitfield FidnNextClear Exit")
			return i
		}
	}
	//log.Println("Bitfield FidnNextClear Exit")
	return -1
}*/

/*func (b *Bitfield) AndNot(b2 *Bitfield) {
	b.mutex.Lock()
	//log.Println("Bitfield AndNot")
	defer b.mutex.Unlock()
	if b.n != b2.n {
		panic("Unequal bitset sizes")
	}
	for i := 0; i < len(b.b); i++ {
		b.b[i] = b.b[i] & ^b2.b[i]
	}
	b.clearEnd()
	//log.Println("Bitfield AndNot Exit")
}*/

/*func (b *Bitfield) clearEnd() {
	// Since clearend is only used from AndNot, no need to get the Lock
	//b.mutex.Lock()
	//log.Println("Bitfield clearEnd")
	//defer b.mutex.Unlock()
	if b.endIndex >= 0 {
		b.b[b.endIndex] &= b.endMask
	}
	//log.Println("Bitfield clearEnd Exit")
}*/

/*func (b *Bitfield) IsEndValid() bool {
	b.mutex.RLock()
	//log.Println("Bitfield ")
	defer b.mutex.RUnlock()
	if b.endIndex >= 0 {
		//log.Println("Bitfield  Exit")
		return (b.b[b.endIndex] & b.endMask) == 0
	}
	//log.Println("Bitfield  Exit")
	return true
}*/

/*func (b *Bitfield) Clear(index int64) {
	b.mutex.Lock()
	//log.Println("Bitfield Clear")
	defer b.mutex.Unlock()
	if index < 0 || index >= b.n {
		panic("Index out of range.")
	}
	b.b[index>>3] &= ^byte(128 >> byte(index&7))
	b.done--
	//log.Println("Bitfield Clear Exit")
}*/