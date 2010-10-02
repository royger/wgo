package wgo_io

import(
	"os"
	"io"
	)

type multiReaderAt struct {
	files   []*os.File
	offsets []int64
	sizes   []int64  
}

// Find the file that matches the offset
func (mr *multiReaderAt) find(offset int64) int {
	// Binary search
	low := 0
	high := len(mr.offsets)
	for low < high-1 {
		probe := (low + high) / 2
		entry := mr.offsets[probe]
		if offset < entry {
			high = probe
		} else {
			low = probe
		}
	}
	return low
}

// Read a block starting from a certain offset

func (mr *multiReaderAt) ReadAt(p []byte, off int64) (n int, err os.Error) {
	for index := mr.find(off); len(p) > 0 && index < len(mr.offsets); index++ {
		chunk := int64(len(p))
		//entry := &mr.files[index]
		itemOffset := off - mr.offsets[index]
		if itemOffset < mr.sizes[index] {
			space := mr.sizes[index] - itemOffset
			if space < chunk {
				chunk = space
			}
			nThisTime, err := mr.files[index].ReadAt(p[0:chunk], itemOffset)
			n = n + nThisTime
			if err != nil {
				return
			}
			p = p[nThisTime:]
			off += int64(nThisTime)
		}
	}
	// At this point if there's anything left to read it means we've run off the
	// end of the file store. Read zeros. This is defined by the bittorrent protocol.
	for i, _ := range (p) {
		p[i] = 0
		n++
	}
	return
}

// MultiReaderAt returns a ReaderAt that's the logical concatenation of
// the provided input files.
func MultiReaderAt(files []*os.File) (io.ReaderAt, os.Error) {
	mr := &multiReaderAt{files, make([]int64, len(files)), make([]int64, len(files))}
	offset := int64(0)
	for i, file := range files {
		mr.offsets[i] = offset
		info, err := file.Stat()
		if err != nil {
			return mr, err
		}
		mr.sizes[i] = info.Size
		offset += info.Size
	}
	return mr, nil
}
