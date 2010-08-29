package main

import(
	"os"
	"sync"
	//"log"
	)

// As defined by the bittorrent protocol, this bitset is big-endian, such that
// the high bit of the first byte is block 0

/*const(
	set = iota
	isset
	bits
	length
	hasmorepieces
	count
	completed
)*/

/*type bitfieldMsg struct {
	method int
	index int64
	bitfield *Bitfield
	bool_response bool
	int_response int64
	byte_response []byte
}*/

type Bitfield struct {
	b        []byte
	n        int64
	done     int64
	endIndex int64
	endMask  byte // Which bits of the last byte are valid
	mutex *sync.RWMutex
	//requests chan *bitfieldMsg
	//responses chan *bitfieldMsg
}

func NewBitfield(n int64) (bitfield *Bitfield) {
	endIndex, endOffset := n>>3, n&7
	endMask := ^byte(255 >> byte(endOffset))
	if endOffset == 0 {
		endIndex = -1
	}
	bitfield = &Bitfield{make([]byte, (n+7)>>3), n, 0, endIndex, endMask, new(sync.RWMutex)/*, make(chan *bitfieldMsg), make(chan *bitfieldMsg)*/}
	//go bitfield.run()
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

/*func (b *Bitfield) run() {
	for {
		r := <- b.requests
		switch r.method {
			case set:
				b.set(r.index)
			case isset:
				r.bool_response = b.isSet(r.index)
			case bits:
				r.byte_response = b.bytes()
			case length:
				r.int_response = b.len()
			case hasmorepieces:
				r.bool_response = b.hasMorePieces(r.bitfield)
			case count:
				r.int_response = b.count()
			case completed:
				r.bool_response = b.completed()
		}
		b.responses <- r
	}
}*/

func (b *Bitfield) Set(index int64) {
	b.mutex.Lock()
	//log.Stderr("Bitfield Set")
	defer b.mutex.Unlock()
	if index < 0 || index >= b.n {
		panic("Index out of range.")
	}
	b.b[index>>3] |= byte(128 >> byte(index&7))
	b.done++
	//log.Stderr("Bitfield Set Exit")
	return
}

/*func (b *Bitfield) Set(index int64) {
	r := new(bitfieldMsg)
	r.method = set
	r.index = index
	b.requests <- r
	<- b.responses
}*/

func (b *Bitfield) IsSet(index int64) bool {
	//log.Stderr("Trying Bitfield IsSet")
	b.mutex.RLock()
	//log.Stderr("Bitfield IsSet")
	defer b.mutex.RUnlock()
	if index < 0 || index >= b.n {
		panic("Index out of range.")
	}
	//log.Stderr("Bitfield IsSet Exit")
	//result = (b.b[index>>3] & byte(128>>byte(index&7))) != 0
	return (b.b[index>>3] & byte(128>>byte(index&7))) != 0
}

/*func (b *Bitfield) IsSet(index int64) (bool) {
	r := new(bitfieldMsg)
	r.method = isset
	r.index = index
	b.requests <- r
	r = <- b.responses
	return r.bool_response
}*/

func (b *Bitfield) Bytes() []byte {
	b.mutex.RLock()
	//log.Stderr("Bitfield Bytes")
	defer b.mutex.RUnlock()
	//bitfield = b.b
	//log.Stderr("Bitfield Bytes Exit")
	return b.b
}

/*func (b *Bitfield) Bytes() ([]byte) {
	r := new(bitfieldMsg)
	r.method = bits
	b.requests <- r
	r = <- b.responses
	return r.byte_response
}*/

func (b *Bitfield) Len() int64 {
	b.mutex.RLock()
	//log.Stderr("Bitfield Len")
	defer b.mutex.RUnlock()
	//log.Stderr("Bitfield Len Exit")
	return b.n
}

/*func (b *Bitfield) Len() int64 {
	r := new(bitfieldMsg)
	r.method = length
	b.requests <- r
	r = <- b.responses
	return r.int_response
}*/

func (b *Bitfield) HasMorePieces(p *Bitfield) bool {
	b.mutex.RLock()
	//log.Stderr("Bitfield HasMorePieces")
	defer b.mutex.RUnlock()
	for i := int64(0); i < b.n; i++ {
		if !((b.b[i>>3] & byte(128>>byte(i&7))) != 0) && ((p.b[i>>3] & byte(128>>byte(i&7))) != 0) {
			//log.Stderr("Bitfield HasMorePieces Exit")
			return true
		}
	}
	//log.Stderr("Bitfield HasMorePieces Exit")
	return false
	
}

/*func (b *Bitfield) HasMorePieces(p *Bitfield) bool {
	r := new(bitfieldMsg)
	r.method = hasmorepieces
	r.bitfield = p
	b.requests <- r
	r = <- b.responses
	return r.bool_response
}*/

func (b *Bitfield) Count() int64 {
	//log.Stderr("Trying Bitfield Count")
	b.mutex.RLock()
	//log.Stderr("Bitfield Count")
	defer b.mutex.RUnlock()
	/*for i := int64(0); i < b.n; i++ {
		if b.IsSet(i) {
			num++
		}
	}*/
	//num = b.done
	//log.Stderr("Bitfield Count Exit")
	return b.done
}

/*func (b *Bitfield) Count() (num int64) {
	r := new(bitfieldMsg)
	r.method = count
	b.requests <- r
	r = <- b.responses
	return r.int_response
}*/

func (b *Bitfield) Completed() bool {
	//log.Stderr("Trying Bitfield Completed")
	b.mutex.RLock()
	//log.Stderr("Bitfield Completed")
	defer b.mutex.RUnlock()
	if b.done == b.n {
		//log.Stderr("Bitfield Completed Exit")
		return true
	}
	//log.Stderr("Bitfield Completed Exit")
	return false
}

/*func (b *Bitfield) Completed() bool {
	r := new(bitfieldMsg)
	r.method = completed
	b.requests <- r
	r = <- b.responses
	return r.bool_response
}*/

// TODO: Make this fast
/*func (b *Bitfield) FindNextSet(index int64) int64 {
	b.mutex.RLock()
	//log.Stderr("Bitfield FindNextSet")
	defer b.mutex.RUnlock()
	for i := index; i < b.n; i++ {
		if (b.b[i>>3] & byte(128>>byte(i&7))) != 0 {
			//log.Stderr("Bitfield FindNextSet Exit")
			return i
		}
	}
	//log.Stderr("Bitfield FindNExtSet Exit")
	return -1
}*/

// TODO: Make this fast
/*func (b *Bitfield) FindNextClear(index int64) int64 {
	b.mutex.RLock()
	//log.Stderr("Bitfield FidnNextClear")
	defer b.mutex.RUnlock()
	for i := index; i < b.n; i++ {
		if (b.b[i>>3] & byte(128>>byte(i&7))) == 0 {
			//log.Stderr("Bitfield FidnNextClear Exit")
			return i
		}
	}
	//log.Stderr("Bitfield FidnNextClear Exit")
	return -1
}*/

/*func (b *Bitfield) AndNot(b2 *Bitfield) {
	b.mutex.Lock()
	//log.Stderr("Bitfield AndNot")
	defer b.mutex.Unlock()
	if b.n != b2.n {
		panic("Unequal bitset sizes")
	}
	for i := 0; i < len(b.b); i++ {
		b.b[i] = b.b[i] & ^b2.b[i]
	}
	b.clearEnd()
	//log.Stderr("Bitfield AndNot Exit")
}*/

/*func (b *Bitfield) clearEnd() {
	// Since clearend is only used from AndNot, no need to get the Lock
	//b.mutex.Lock()
	//log.Stderr("Bitfield clearEnd")
	//defer b.mutex.Unlock()
	if b.endIndex >= 0 {
		b.b[b.endIndex] &= b.endMask
	}
	//log.Stderr("Bitfield clearEnd Exit")
}*/

/*func (b *Bitfield) IsEndValid() bool {
	b.mutex.RLock()
	//log.Stderr("Bitfield ")
	defer b.mutex.RUnlock()
	if b.endIndex >= 0 {
		//log.Stderr("Bitfield  Exit")
		return (b.b[b.endIndex] & b.endMask) == 0
	}
	//log.Stderr("Bitfield  Exit")
	return true
}*/

/*func (b *Bitfield) Clear(index int64) {
	b.mutex.Lock()
	//log.Stderr("Bitfield Clear")
	defer b.mutex.Unlock()
	if index < 0 || index >= b.n {
		panic("Index out of range.")
	}
	b.b[index>>3] &= ^byte(128 >> byte(index&7))
	b.done--
	//log.Stderr("Bitfield Clear Exit")
}*/