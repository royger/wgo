package main

import(
	"os"
	)

// As defined by the bittorrent protocol, this bitset is big-endian, such that
// the high bit of the first byte is block 0

type Bitfield struct {
	b        []byte
	n        int
	endIndex int
	endMask  byte // Which bits of the last byte are valid
}

func NewBitfield(n int) (bitfield *Bitfield) {
	endIndex, endOffset := n>>3, n&7
	endMask := ^byte(255 >> byte(endOffset))
	if endOffset == 0 {
		endIndex = -1
	}
	bitfield = &Bitfield{make([]byte, (n+7)>>3), n, endIndex, endMask}
	return
}

// Creates a new bitset from a given byte stream. Returns nil if the
// data is invalid in some way.
func NewBitfieldFromBytes(n int, data []byte) (bitfield *Bitfield, err os.Error) {
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

func (b *Bitfield) Set(index int) {
	if index < 0 || index >= b.n {
		panic("Index out of range.")
	}
	b.b[index>>3] |= byte(128 >> byte(index&7))
}

func (b *Bitfield) Clear(index int) {
	if index < 0 || index >= b.n {
		panic("Index out of range.")
	}
	b.b[index>>3] &= ^byte(128 >> byte(index&7))
}

func (b *Bitfield) IsSet(index int) bool {
	if index < 0 || index >= b.n {
		panic("Index out of range.")
	}
	return (b.b[index>>3] & byte(128>>byte(index&7))) != 0
}

func (b *Bitfield) AndNot(b2 *Bitfield) {
	if b.n != b2.n {
		panic("Unequal bitset sizes")
	}
	for i := 0; i < len(b.b); i++ {
		b.b[i] = b.b[i] & ^b2.b[i]
	}
	b.clearEnd()
}

func (b *Bitfield) clearEnd() {
	if b.endIndex >= 0 {
		b.b[b.endIndex] &= b.endMask
	}
}

func (b *Bitfield) IsEndValid() bool {
	if b.endIndex >= 0 {
		return (b.b[b.endIndex] & b.endMask) == 0
	}
	return true
}

// TODO: Make this fast
func (b *Bitfield) FindNextSet(index int) int {
	for i := index; i < b.n; i++ {
		if (b.b[i>>3] & byte(128>>byte(i&7))) != 0 {
			return i
		}
	}
	return -1
}

// TODO: Make this fast
func (b *Bitfield) FindNextClear(index int) int {
	for i := index; i < b.n; i++ {
		if (b.b[i>>3] & byte(128>>byte(i&7))) == 0 {
			return i
		}
	}
	return -1
}
