package bencode

import(
	"os"
	"io"
	"fmt"
	"bufio"
	"strconv"
	"crypto/sha1"
	"encoding/binary"
)

type Torrent struct {
	Infohash string
	Announce string
	Announce_list []string
	Creation int64
	Comment string
	Created_by string
	Encoding string
	Info Info
}

type Info struct {
	Length int64
	Name string
	Piece_length int64
	Private bool
	Pieces []byte
	Md5sum []byte
	Files []File
	Source string
}

type File struct {
	Length int64
	Path []string
	Md5sum []byte
}

type Tracker struct {
	Failure_reason string
	Warning_message string
	Interval int64
	Min_interval int64
	Tracker_id string
	Complete int64
	Incomplete int64
	Downloaded int64
	Peers []Peer
}

type Peer struct {
	Peer_id string
	Ip string
	Port int64
}

type Input struct {
	file_buf *bufio.Reader
	file io.ReadCloser
}

const(
	integer = 'i'
	list = 'l'
	dictionary = 'd'
	end = 'e'
	str = 's'
)

func (torrent *Input) getString() (string) {
	i, err := torrent.file_buf.ReadBytes(':')
	if err != nil {
		panic("getString: could not read length of string: " + err.String())
	}
	l, err := strconv.Atoi64(string(i[0:len(i)-1]))
	if err != nil {
		panic("getString: convert ascii to uint failed: " + err.String())
	}
	b := make([]byte, l)
	n, err := torrent.file_buf.Read(b)
	if err != nil || int64(n) != l {
		panic("getString: could not read string " + err.String())
	}
	return string(b)
}

func (torrent *Input) match(c byte) (e os.Error) {
	b, err := torrent.file_buf.ReadByte()
	if err != nil {
		panic("match: " + err.String())
	}
	if c == str {
		if '0' <= b && b <= '9' {
			torrent.file_buf.UnreadByte()
		} else {
			torrent.file_buf.UnreadByte()
			e = os.NewError("match: expecting " + string(c) + " found " + string(b))
		}
	} else if b != c {
		torrent.file_buf.UnreadByte()
		e = os.NewError("match: expecting " + string(c) + " found " + string(b))
	}
	return
}

func (torrent *Input) getInteger() (int64) {
	n, err := torrent.file_buf.ReadBytes('e')
	if err != nil {
		panic("getString: could not read int " + err.String())
	}
	i, err := strconv.Atoi64(string(n[0:len(n)-1]))
	if err != nil {
		panic("getInteger: convert ascii to uint failed: " + err.String())
	}
	return i
}

func (torrent *Input) scanTorrent(metainfo *Torrent) (err os.Error) {
	torrent.match(dictionary)
	for err = torrent.match(str); err == nil; err = torrent.match(str) {
		s := torrent.getString()
		switch s {
			case "announce":
				metainfo.Announce = torrent.getString()
				metainfo.Announce_list = appendString(metainfo.Announce_list, metainfo.Announce)
			case "comment":
				metainfo.Comment = torrent.getString()
			case "created by":
				metainfo.Created_by = torrent.getString()
			case "creation date":
				if e := torrent.match(integer); e != nil {
					err = os.NewError("scan: match failed " + e.String())
					return
				}
				metainfo.Creation = torrent.getInteger()
			case "encoding":
				metainfo.Encoding = torrent.getString()
			case "announce-list":
				torrent.match(list)
				for e := torrent.match(list); e == nil; e = torrent.match(list) {
					metainfo.Announce_list = appendString(metainfo.Announce_list, torrent.getString())
					torrent.match(end)
				}
				torrent.match(end)
			case "info":
				if e := torrent.match(dictionary); e != nil {
					err = os.NewError("scan: match failed " + e.String())
					return
				}
				metainfo.Infohash += "d"
				for e := torrent.match(str); e == nil; e = torrent.match(str) {
					switch torrent.getString() {
						case "length":
							if e := torrent.match(integer); e != nil {
								err = os.NewError("scan: match failed " + e.String())
								return
							}
							metainfo.Info.Length = torrent.getInteger()
							metainfo.Infohash += "6:lengthi" + strconv.Itoa64(metainfo.Info.Length) + "e"
						case "name":
							metainfo.Info.Name = torrent.getString()
							metainfo.Infohash += "4:name" + strconv.Itoa(len(metainfo.Info.Name)) + ":" + metainfo.Info.Name 
						case "piece length":
							if e := torrent.match(integer); e != nil {
								err = os.NewError("scan: match failed " + e.String())
								return
							}
							metainfo.Info.Piece_length = torrent.getInteger()
							metainfo.Infohash += "12:piece lengthi" + strconv.Itoa64(metainfo.Info.Piece_length) + "e"
						case "private":
							if e := torrent.match(integer); e != nil {
								err = os.NewError("scan: match failed " + e.String())
								return
							}
							p := torrent.getInteger()
							if p == 1 {
								metainfo.Info.Private = true
							}
							metainfo.Infohash += "7:privatei" + strconv.Itoa64(p) + "e"
						case "pieces":
							p := torrent.getString()
							metainfo.Info.Pieces = []byte(p)
							metainfo.Infohash += "6:pieces" + strconv.Itoa(len(p)) + ":" + p
						case "md5sum":
							p := torrent.getString()
							metainfo.Info.Md5sum = []byte(p)
							metainfo.Infohash += "6:md5sum" + strconv.Itoa(len(p)) + ":" + p
						case "source":
							metainfo.Info.Source = torrent.getString()
							metainfo.Infohash += "6:source" + strconv.Itoa(len(metainfo.Info.Source)) + ":" + metainfo.Info.Source
						case "files":
							// Multifile Torrent
							torrent.match(list)
							metainfo.Infohash += "5:filesl"
							for i, e := 0, torrent.match(dictionary); e == nil; i, e = i+1, torrent.match(dictionary) {
								var p []string
								var l int64
								var m []byte
								metainfo.Infohash += "d"
								for e := torrent.match(str); e == nil; e = torrent.match(str) {
									switch torrent.getString() {
										case "length":
											if e := torrent.match(integer); e != nil {
												err = os.NewError("scan: match failed " + e.String())
												return
											}
											l = torrent.getInteger()
											metainfo.Infohash += "6:lengthi" + strconv.Itoa64(l) + "e"
										case "path":
											p = make([]string, 0)
											torrent.match(list)
											metainfo.Infohash += "4:pathl"
											for i, e := 0, torrent.match(str); e == nil; i, e = i+1, torrent.match(str) {
												s := torrent.getString()
												p = appendString(p, s)
												metainfo.Infohash += strconv.Itoa(len(s)) + ":" + s
											}
											torrent.match(end)
											metainfo.Infohash += "e"
										case "md5sum":
											s := torrent.getString()
											m = []byte(s)
											metainfo.Infohash += "6:md5sum" + strconv.Itoa(len(s)) + ":" + s
									}
								}
								torrent.match(end)
								metainfo.Infohash += "e"
								metainfo.Info.Files = appendFile(metainfo.Info.Files, File{Length: l, Path: p, Md5sum: m})
							}
							torrent.match(end)
							metainfo.Infohash += "e"
					}
				}
				torrent.match(end)
				metainfo.Infohash += "e"
			}
		}
	err = torrent.match(end)
	return
}

func (input *Input) scanTracker(tracker *Tracker) (err os.Error) {
	input.match(dictionary)
	for err = input.match(str); err == nil; err = input.match(str) {
		switch input.getString() {
			case "failure reason":
				input.match(str)
				tracker.Failure_reason = input.getString()
			case "warning message":
				input.match(str)
				tracker.Warning_message = input.getString()
			case "interval":
				input.match(integer)
				tracker.Interval = input.getInteger()
			case "min interval":
				input.match(integer)
				tracker.Min_interval = input.getInteger()
			case "tracker id":
				input.match(str)
				tracker.Tracker_id = input.getString()
			case "complete":
				input.match(integer)
				tracker.Complete = input.getInteger()
			case "incomplete":
				input.match(integer)
				tracker.Incomplete = input.getInteger()
			case "downloaded":
				input.match(integer)
				tracker.Downloaded = input.getInteger()
			case "peers":
				if err = input.match(list); err == nil {
					for err = input.match(dictionary); err == nil; err = input.match(dictionary) {
						var id string
						var ip string
						var port int64
						for err = input.match(str); err == nil; err = input.match(str) {
							switch input.getString() {
								case "peer id":
									input.match(str)
									id = input.getString()
								case "ip":
									input.match(str)
									ip = input.getString()
								case "port":
									input.match(integer)
									port = input.getInteger()
							}
						}
						tracker.Peers = appendPeer(tracker.Peers, Peer{Peer_id: id, Ip: ip, Port: port})
						input.match(end)
					}
					input.match(end)
				} else {
					input.match(str)
					peers := []byte(input.getString())
					for i := 0; i < len(peers); i = i+6 {
						ip := fmt.Sprintf("%d.%d.%d.%d", peers[i+0], peers[i+1], peers[i+2], peers[i+3])
						port := int64(binary.BigEndian.Uint16(peers[i+4:i+6]))
						tracker.Peers = appendPeer(tracker.Peers, Peer{Ip: ip, Port: port})
					}
				}
			}
	}
	err = input.match(end)
	return
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

func appendFile(slice []File, data File) []File {
	l := len(slice)
	if l + 1 > cap(slice) {  // reallocate
		// Allocate 10 more slots
		newSlice := make([]File, (l+10))
		// The copy function is predeclared and works for any slice type.
		copy(newSlice, slice)
		slice = newSlice
	}
	slice = slice[0:l+1]
	slice[l] = data
	return slice
}

func appendPeer(slice []Peer, data Peer) []Peer {
	l := len(slice)
	if l + 1 > cap(slice) {  // reallocate
		// Allocate 10 more slots
		newSlice := make([]Peer, (l+10))
		// The copy function is predeclared and works for any slice type.
		copy(newSlice, slice)
		slice = newSlice
	}
	slice = slice[0:l+1]
	slice[l] = data
	return slice
}

func Parse(file io.ReadCloser) (metainfo *Torrent, err os.Error) {
	metainfo = new(Torrent)
	metainfo.Announce_list = make([]string, 0, 10)
	metainfo.Info.Files = make([]File, 0, 10)
	torrent := new(Input)
	torrent.file = file
	torrent.file_buf = bufio.NewReader(torrent.file)
	err = torrent.scanTorrent(metainfo)
	hash := sha1.New()
	hash.Write([]byte(metainfo.Infohash))
	metainfo.Infohash = string(hash.Sum())
	return
}

func ParseTracker(file io.ReadCloser) (tracker *Tracker, err os.Error) {
	tracker = new(Tracker)
	tracker.Peers = make([]Peer, 0, 10)
	response := new(Input)
	response.file = file
	response.file_buf = bufio.NewReader(response.file)
	err = response.scanTracker(tracker)
	return
}
