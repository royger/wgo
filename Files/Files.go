// Create the structure of the files to download in the FS
// This file in mainly the files.go found in Taipei-Torrent with some changes
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package files

import(
	"io"
	"os"
	"strings"
	"log"
	"crypto/sha1"
	"bytes"
	"wgo/bencode"
	"wgo/wgo_io"
	"wgo/bit_field"
	"sync"
	)

const(
	readat = iota
	writeat
	checkpiece
)

const(
	FILE_PERM = 0666
	FOLDER_PERM = 0755
	HASHERS = 5
)

type Files interface {
	GetReaderAt(index, begin, length int64) (io.Reader)
	WriteAt(index, begin int64, bytes []byte) (os.Error)
	CheckPiece(index int64) (os.Error)
	CheckPieces() (left int64, bf *bit_field.Bitfield, err os.Error)
}

type fileEntry struct {
	length int64
	fd     *os.File
}

type fileStore struct {
	mutex *sync.Mutex
	offsets []int64
	totalLength int64
	files   []fileEntry // Stored in increasing globalOffset order
	info *bencode.InfoDict
	reader io.ReaderAt 
}

type CheckPiece struct {
	index int64
	ok bool
	err os.Error
}

func (fe *fileStore) GetReaderAt(index, begin, length int64) (reader io.Reader) {
	fe.mutex.Lock()
	defer fe.mutex.Unlock()
	globalOffset := index*fe.info.Piece_length + begin
	return io.NewSectionReader(fe.reader, globalOffset, length)
}

func (fe *fileStore) WriteAt(indexp, begin int64, bytes []byte) (err os.Error){
	fe.mutex.Lock()
	defer fe.mutex.Unlock()
	var n int
	off := indexp*fe.info.Piece_length + begin
	//_, err = fe.writeAt(bytes, globalOffset)
	index := fe.find(off)
	for len(bytes) > 0 && index < len(fe.offsets) {
		chunk := int64(len(bytes))
		entry := &fe.files[index]
		itemOffset := int64(off - fe.offsets[index])
		if itemOffset < entry.length {
			space := int64(entry.length - itemOffset)
			if space < chunk {
				chunk = space
			}
			fd := entry.fd
			nThisTime, err := fd.WriteAt(bytes[0:chunk], itemOffset)
			n += nThisTime
			if err != nil {
				return
			}
			bytes = bytes[nThisTime:]
			off += int64(nThisTime)
		}
		index++
	}
	// At this point if there's anything left to write it means we've run off the
	// end of the file store. Check that the data is zeros.
	// This is defined by the bittorrent protocol.
	for i, _ := range (bytes) {
		if bytes[i] != 0 {
			err = os.NewError("Unexpected non-zero data at end of store.")
			n = n + i
			return
		}
	}
	n = n + len(bytes)
	return
}

func (fe *fileStore) CheckPiece(index int64) (os.Error) {
	fe.mutex.Lock()
	defer fe.mutex.Unlock()
	return fe.checkPiece(index)
}

func (fe *fileEntry) open(name string, length int64) (err os.Error) {
	fe.length = length
	fe.fd, err = os.Open(name, os.O_RDWR|os.O_CREAT, FILE_PERM)
	if err != nil {
		return
	}
	if err = fe.fd.Truncate(length); err != nil {
		return
	}
	return
}

func NewFiles(info *bencode.InfoDict, fileDir string) (f Files, totalSize int64, err os.Error) {
	fs := new(fileStore)
	fs.mutex = new(sync.Mutex)
	fs.info = info
	//log.Println(info)
	numFiles := len(info.Files)
	log.Println("Files -> Number of files:", numFiles)
	if numFiles == 0 {
		// Create dummy Files structure.
		info = &bencode.InfoDict{Files: []bencode.FileDict{bencode.FileDict{Length: info.Length, Path: []string{info.Name}, Md5sum: info.Md5sum}}}
		numFiles = 1
	} else {
		fileDir = fileDir + "/" + info.Name
	}
	fs.files = make([]fileEntry, numFiles)
	fs.offsets = make([]int64, numFiles)
	for i, _ := range (info.Files) {
		src := &info.Files[i]
		//log.Println("Files ->", src.Path)
		torrentPath, err := joinPath(src.Path)
		if err != nil {
			log.Println("Files ->",err)
			return
		}
		//log.Println("Files ->", torrentPath)
		if err != nil {
			log.Println("Files ->",err)
			return fs, 0, err 
		}
		fullPath := fileDir + "/" + torrentPath
		err = ensureDirectory(fullPath)
		//log.Println("Files -> Fullpath:", fullPath)
		if n := strings.LastIndex(fullPath, "/"); n != -1 {
			if err = os.MkdirAll(fullPath[0:n], FOLDER_PERM); err != nil {
				log.Println("Files ->",err)
				return fs, 0, err
			}
		}
		if err != nil {
			log.Println("Files ->",err)
			return fs, 0, err
		}
		err = fs.files[i].open(fullPath, src.Length)
		if err != nil {
			log.Println("Files ->",err)
			return fs, 0, err
		}
		fs.offsets[i] = totalSize
		totalSize += src.Length
	}
	fs.totalLength = totalSize
	files := make([]*os.File, numFiles)
	for i, file := range fs.files {
		files[i] = file.fd
	}
	fs.reader, err = wgo_io.MultiReaderAt(files)
	if err != nil {
		return
	}
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

func (fs *fileStore) CheckPieces() (left int64, bf *bit_field.Bitfield, err os.Error) {
	numPieces := (fs.totalLength + fs.info.Piece_length - 1) / fs.info.Piece_length
	log.Println("Files -> totalLength:", fs.totalLength, "pieceLength:", fs.info.Piece_length, "numPieces:", numPieces)
	log.Println("Files -> Checking pieces")
	bf = bit_field.NewBitfield(numPieces)
	input := make(chan *CheckPiece, HASHERS)
	output := make(chan *CheckPiece, HASHERS)
	for i := int64(0); i < HASHERS; i++ {
		go func(i int64, output, input chan *CheckPiece) {
			for piece := range input {
				piece.err = fs.checkPiece(piece.index)
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
			if piece.index == numPieces-1 {
				left += fs.totalLength-piece.index*fs.info.Piece_length
			} else {
				left += fs.info.Piece_length
			}
		} else {
			bf.Set(piece.index)
		}
	}
	close(output)
	return
}
// Check a piece

func (fs *fileStore) checkPiece(pieceIndex int64) (err os.Error) {
	ref := fs.info.Pieces
	currentSum, err := fs.computePieceSum(pieceIndex)
	if err != nil {
		return
	}
	base := pieceIndex * sha1.Size
	end := base + sha1.Size
	if !bytes.Equal([]byte(ref[base:end]), currentSum) {
		err = os.NewError("Piece hash doesn't match")
	}
	return
}


func (fs *fileStore) computePieceSum(pieceIndex int64) (sum []byte, err os.Error) {
	numPieces := (fs.totalLength + fs.info.Piece_length - 1) / fs.info.Piece_length
	hasher := sha1.New()
	length := fs.info.Piece_length
	if pieceIndex == numPieces-1 {
		length = fs.totalLength-pieceIndex*fs.info.Piece_length
	}
	reader := io.NewSectionReader(fs.reader, pieceIndex*fs.info.Piece_length, length)
	_, err = io.Copy(hasher, reader)
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

