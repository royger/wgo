// Create the structure of the files to download in the FS
// This file in mainly the files.go found in Taipei-Torrent with some changes
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package main

import(
	"io"
	"os"
	"strings"
	"syscall"
	"log"
	"crypto/sha1"
	"bytes"
	"wgo/bencode"
	)

const(
	readat = iota
	writeat
	checkpiece
)

type FileStore interface {
	io.ReaderAt
	io.WriterAt
	io.Closer
	CheckPieces() (left int64, goodBits *Bitfield, err os.Error)
	CheckPiece(pieceIndex int64) (good bool, err os.Error) 
	ComputePieceSum(pieceIndex int64) (sum []byte, err os.Error)
	Run()
}

type FileStoreMsg struct {
	Id int
	Index int64
	Begin int64
	//Length int64
	Bytes []byte
	Ok bool
	Response chan *FileStoreMsg
	Err os.Error
}

type fileEntry struct {
	length int64
	fd     *os.File
}

type fileStore struct {
	offsets []int64
	totalLength int64
	files   []fileEntry // Stored in increasing globalOffset order
	info *bencode.Info
	incoming chan *FileStoreMsg
}

type CheckPiece struct {
	index int64
	ok bool
	err os.Error
}

func (fe *fileEntry) open(name string, length int64) (err os.Error) {
	fe.length = length
	fe.fd, err = os.Open(name, os.O_RDWR|os.O_CREAT, FILE_PERM)
	if err != nil {
		return
	}
	errno := syscall.Truncate(name, length)
	if errno != 0 {
		err = os.NewError("Could not truncate file.")
	}
	return
}

func (fe *fileStore) Run() {
	for {
		select {
			case msg := <- fe.incoming:
				switch msg.Id {
					case readat:
						// Read and send the piece
						//b := make([]byte, msg.Length)
						globalOffset := msg.Index*fe.info.Piece_length + msg.Begin
						n, err := fe.ReadAt(msg.Bytes, globalOffset)
						if err != nil {
							msg.Err = err
							msg.Response <- msg
							break
						}
						if n != len(msg.Bytes) {
							msg.Err = os.NewError("Readed length is different than expected")
							msg.Response <- msg
							break
						}
						msg.Ok = true
						msg.Response <- msg
					case writeat:
						// Write the piece to disc
						globalOffset := msg.Index*fe.info.Piece_length + msg.Begin
						n, err := fe.WriteAt(msg.Bytes, globalOffset)
						if err != nil {
							msg.Err = err
							msg.Response <- msg
							break
						}
						if n != len(msg.Bytes) {
							msg.Err = os.NewError("Written length is different than expected")
							msg.Response <- msg
							break
						}
						msg.Ok = true
						msg.Response <- msg
					case checkpiece:
						// Check the piece and return the result
						msg.Ok, msg.Err = fe.CheckPiece(msg.Index)
						msg.Response <- msg
			}
		}
	}
}

func NewFileStore(info *bencode.Info, fileDir string, incoming chan *FileStoreMsg) (f FileStore, totalSize int64, err os.Error) {
	fs := new(fileStore)
	fs.info = info
	numFiles := len(info.Files)
	fs.incoming = incoming
	if numFiles == 0 {
		// Create dummy Files structure.
		info = &bencode.Info{Files: []bencode.File{bencode.File{Length: info.Length, Path: []string{info.Name}, Md5sum: info.Md5sum}}}
		numFiles = 1
	}
	fs.files = make([]fileEntry, numFiles)
	fs.offsets = make([]int64, numFiles)
	for i, _ := range (info.Files) {
		src := &info.Files[i]
		log.Stderr(src.Path)
		torrentPath, err := joinPath(src.Path)
		if err != nil {
			return
		}
		log.Stderr("File", torrentPath)
		if err != nil {
			return fs, 0, err 
		}
		fullPath := fileDir + "/" + torrentPath
		err = ensureDirectory(fullPath)
		if err != nil {
			return fs, 0, err
		}
		err = fs.files[i].open(fullPath, src.Length)
		if err != nil {
			return fs, 0, err
		}
		fs.offsets[i] = totalSize
		totalSize += src.Length
	}
	fs.totalLength = totalSize
	f = fs
	return
}

// Find the file that matches the offset

func (f *fileStore) find(offset int64) int {
	// Binary search
	offsets := f.offsets
	low := 0
	high := len(offsets)
	for low < high-1 {
		probe := (low + high) / 2
		entry := offsets[probe]
		if offset < entry {
			high = probe
		} else {
			low = probe
		}
	}
	return low
}

// Read a block starting from a certain offset

func (f *fileStore) ReadAt(p []byte, off int64) (n int, err os.Error) {
	index := f.find(off)
	for len(p) > 0 && index < len(f.offsets) {
		chunk := int64(len(p))
		entry := &f.files[index]
		itemOffset := off - f.offsets[index]
		if itemOffset < entry.length {
			space := entry.length - itemOffset
			if space < chunk {
				chunk = space
			}
			fd := entry.fd
			nThisTime, err := fd.ReadAt(p[0:chunk], itemOffset)
			n = n + nThisTime
			if err != nil {
				return
			}
			p = p[nThisTime:]
			off += int64(nThisTime)
		}
		index++
	}
	// At this point if there's anything left to read it means we've run off the
	// end of the file store. Read zeros. This is defined by the bittorrent protocol.
	for i, _ := range (p) {
		p[i] = 0
		n++
	}
	return
}

// Write a block starting at offset

func (f *fileStore) WriteAt(p []byte, off int64) (n int, err os.Error) {
	index := f.find(off)
	for len(p) > 0 && index < len(f.offsets) {
		chunk := int64(len(p))
		entry := &f.files[index]
		itemOffset := off - f.offsets[index]
		if itemOffset < entry.length {
			space := entry.length - itemOffset
			if space < chunk {
				chunk = space
			}
			fd := entry.fd
			nThisTime, err := fd.WriteAt(p[0:chunk], itemOffset)
			n += nThisTime
			if err != nil {
				return
			}
			p = p[nThisTime:]
			off += int64(nThisTime)
		}
		index++
	}
	// At this point if there's anything left to write it means we've run off the
	// end of the file store. Check that the data is zeros.
	// This is defined by the bittorrent protocol.
	for i, _ := range (p) {
		if p[i] != 0 {
			err = os.NewError("Unexpected non-zero data at end of store.")
			n = n + i
			return
		}
	}
	n = n + len(p)
	return
}

func (fs *fileStore) CheckPieces() (left int64, bitfield *Bitfield, err os.Error) {
	numPieces := (fs.totalLength + fs.info.Piece_length - 1) / fs.info.Piece_length
	log.Stderr("totalLength:", fs.totalLength, "pieceLength:", fs.info.Piece_length, "numPieces:", numPieces)
	bitfield = NewBitfield(numPieces)
	input := make(chan *CheckPiece, HASHERS)
	output := make(chan *CheckPiece, HASHERS)
	for i := int64(0); i < HASHERS; i++ {
		go func(i int64, output, input chan *CheckPiece) {
			for piece := range input {
				piece.ok, piece.err = fs.CheckPiece(piece.index)
				output <- piece
			}
		}(i, output, input)
	}
	go func(input chan *CheckPiece) {
		for i:= int64(0); i < numPieces; i++ {
			piece := new(CheckPiece)
			piece.index = i
			input <- piece
		}
		close(input)
	}(input)
	for i:= int64(0); i < numPieces; i++ {
		piece := <- output
		if piece.err != nil {
			err = piece.err
			continue
		}
		if piece.ok {
			bitfield.Set(piece.index)
		} else {
			if piece.index == numPieces-1 {
				left += fs.totalLength-piece.index*fs.info.Piece_length
			} else {
				left += fs.info.Piece_length
			}
		}
	}
	close(output)
	return
}
// Check a piece

func (fs *fileStore) CheckPiece(pieceIndex int64) (good bool, err os.Error) {
	ref := fs.info.Pieces
	currentSum, err := fs.ComputePieceSum(pieceIndex)
	if err != nil {
		return
	}
	base := pieceIndex * sha1.Size
	end := base + sha1.Size
	good = bytes.Equal([]byte(ref[base:end]), currentSum)
	return
}


func (fs *fileStore) ComputePieceSum(pieceIndex int64) (sum []byte, err os.Error) {
	numPieces := (fs.totalLength + fs.info.Piece_length - 1) / fs.info.Piece_length
	hasher := sha1.New()
	piece := make([]byte, fs.info.Piece_length)
	if pieceIndex == numPieces-1 {
		piece = piece[0 : fs.totalLength-pieceIndex*fs.info.Piece_length]
	}
	_, err = fs.ReadAt(piece, pieceIndex*fs.info.Piece_length)
	if err != nil {
		return
	}
	_, err = hasher.Write(piece)
	if err != nil {
		return
	}
	sum = hasher.Sum()
	return
}

// Close all the files in the torrent

func (f *fileStore) Close() (err os.Error) {
	for i, _ := range (f.files) {
		fd := f.files[i].fd
		if fd != nil {
			fd.Close()
			f.files[i].fd = nil
		}
	}
	return
}


// Check that the parts of the path are correct
func joinPath(parts []string) (path string, err os.Error) {
	// TODO: better, OS-specific sanitization.
	for key, part := range (parts) {
		// Sanitize file names.
		if strings.Index(part, "/") >= 0 || strings.Index(part, "\\") >= 0 || part == ".." {
			err = os.NewError("Bad path part " + part)
			return
		}
		// Remove tailing and leading spaces
		if strings.HasPrefix(part, " ") || strings.HasSuffix(part, " ") {
			parts[key] = strings.TrimSpace(part)
		}
	}

	path = strings.Join(parts, "/")
	return
}

// Create the appropiate folders (if needed)
func ensureDirectory(fullPath string) (err os.Error) {
	pathParts := strings.Split(fullPath, "/", 0)
	if len(pathParts) < 2 {
		return
	}
	dirParts := pathParts[0 : len(pathParts)-1]
	path := strings.Join(dirParts, "/")
	err = os.MkdirAll(path, FOLDER_PERM)
	return
}

