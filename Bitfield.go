package main

import(
	"os"
	"sync"
	)

// As defined by the bittorrent protocol, this bitset is big-endian, such that
// the high bit of the first byte is block 0

type Bitfield struct {
	b        []byte
	n        int64
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
	bitfield = &Bitfield{make([]byte, (n+7)>>3), n, endIndex, endMask, new(sync.RWMutex)}
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
	return
}

func (b *Bitfield) Set(index int64) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if index < 0 || index >= b.n {
		panic("Index out of range.")
	}
	b.b[index>>3] |= byte(128 >> byte(index&7))
}

func (b *Bitfield) Clear(index int64) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if index < 0 || index >= b.n {
		panic("Index out of range.")
	}
	b.b[index>>3] &= ^byte(128 >> byte(index&7))
}

func (b *Bitfield) IsSet(index int64) bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	if index < 0 || index >= b.n {
		panic("Index out of range.")
	}
	return (b.b[index>>3] & byte(128>>byte(index&7))) != 0
}

func (b *Bitfield) AndNot(b2 *Bitfield) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if b.n != b2.n {
		panic("Unequal bitset sizes")
	}
	for i := 0; i < len(b.b); i++ {
		b.b[i] = b.b[i] & ^b2.b[i]
	}
	b.clearEnd()
}

func (b *Bitfield) clearEnd() {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if b.endIndex >= 0 {
		b.b[b.endIndex] &= b.endMask
	}
}

func (b *Bitfield) IsEndValid() bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	if b.endIndex >= 0 {
		return (b.b[b.endIndex] & b.endMask) == 0
	}
	return true
}

func (b *Bitfield) Bytes() (bitfield []byte) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	bitfield = b.b
	return
}

func (b *Bitfield) Len() int64 {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.n
}

func (b *Bitfield) HasMorePieces(p *Bitfield) bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	for i := int64(0); i < b.n; i++ {
		if !b.IsSet(i) && p.IsSet(i) {
			return true
		}
	}
	return false
	
}

func (b *Bitfield) Count() (num int64) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	for i := int64(0); i < b.n; i++ {
		if b.IsSet(i) {
			num++
		}
	}
	return
}

func (b *Bitfield) Completed() bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	if b.Count() == b.n {
		return true
	}
	return false
}

// TODO: Make this fast
func (b *Bitfield) FindNextSet(index int64) int64 {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	for i := index; i < b.n; i++ {
		if (b.b[i>>3] & byte(128>>byte(i&7))) != 0 {
			return i
		}
	}
	return -1
}

// TODO: Make this fast
func (b *Bitfield) FindNextClear(index int64) int64 {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	for i := index; i < b.n; i++ {
		if (b.b[i>>3] & byte(128>>byte(i&7))) == 0 {
			return i
		}
	}
	return -1
}
