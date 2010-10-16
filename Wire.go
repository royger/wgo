// Converts RAW messages into something more simple
// to use, and also converts messages structs into
// RAW messages to send to the peers
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package main

import(
	"net"
	"bytes"
	"os"
	"encoding/binary"
	"time"
	"io"
	"bufio"
	)

type Wire struct {
	pstrlen uint8
	pstr string
	reserved []byte
	infohash []byte
	peerid	[]byte
	conn net.Conn
	up_limit *time.Ticker
	down_limit *time.Ticker
	writer *bufio.Writer
	files chan *FileMsg
}
	
type message struct {
	length	uint32
	msgId	uint8
	payLoad	[]byte
	addr	[]string
}

func NewWire(infohash, peerid string, conn net.Conn, up_limit, down_limit *time.Ticker, files chan *FileMsg) (wire *Wire, err os.Error) {
	wire = new(Wire)
	wire.pstr = PROTOCOL
	wire.pstrlen = (uint8)(len(wire.pstr))
	wire.reserved = make([]byte,8)
	wire.infohash = []byte(infohash)
	wire.peerid = []byte(peerid)
	wire.conn = conn
	wire.files = files
	if err = wire.conn.SetTimeout(KEEP_ALIVE_RESP); err != nil {
		return
	}
	wire.writer = bufio.NewWriter(wire.conn)
	wire.up_limit = up_limit
	wire.down_limit = down_limit
	return
}

func (wire *Wire) Handshake() (peerid string, err os.Error) {
	// Sending handshake
	var n int
	
	if err = wire.writer.WriteByte(wire.pstrlen); err != nil {
		return
	}
	if n, err = wire.writer.WriteString(wire.pstr); err != nil || n != len(wire.pstr) {
		return
	}
	if n, err = wire.writer.Write(wire.reserved); err != nil || n != len(wire.reserved) {
		return
	}
	if n, err = wire.writer.Write(wire.infohash); err != nil || n != len(wire.infohash) {
		return
	}
	if n, err = wire.writer.Write(wire.peerid); err != nil || n != len(wire.peerid) {
		return
	}
	if err = wire.writer.Flush(); err != nil {
		return
	}
	// Reading peer handshake
	var header [68]byte
	n, err = io.ReadFull(wire.conn, header[0:1])
	if err != nil || n != 1 {
		return peerid, os.NewError("Reading handshake length: " + err.String())
	}
	if header[0] != 19 {
		return peerid, os.NewError("Invalid length")
	}
	n, err = io.ReadFull(wire.conn, header[1:20])
	if err != nil || n != 19 {
		return peerid, os.NewError("Reading protocol string: " + err.String())
	}
	if string(header[1:20]) != "BitTorrent protocol" {
		return peerid, os.NewError("Unknown protocol")
	}
	// Read rest of header
	n, err = io.ReadFull(wire.conn, header[20:])
	if err != nil || n != len(header[20:]) {
		return peerid, os.NewError("Reading payload of the handshake: " + err.String())
	}
	// See if infohash matches
	if !bytes.Equal(header[28:48], wire.infohash) {
		return peerid, os.NewError("InfoHash doesn't match")
	}
	peerid = string(header[48:68])
	//log.Println("Received header", header)
	return 
}

func (wire *Wire) ReadMsg(piece_buf []byte) (msg *message, err os.Error) {
	var n int
	
	if wire.conn == nil {
		return msg, os.NewError("Invalid connection")
	}
	msg = new(message)
	addr := wire.conn.RemoteAddr()
	if addr == nil {
		return msg, os.NewError("Invalid address")
	}
	msg.addr = []string{addr.String()}
	//var length_header [4]byte
	length_header := make([]byte, 4)
	n, err = io.ReadFull(wire.conn, length_header[0:4]) // read msg length
	if err != nil || n != 4 {
		return msg, os.NewError("Read header length " + err.String())
	}
	msg.length = binary.BigEndian.Uint32(length_header[0:4]) // Convert length
	if msg.length == 0 {
		return // Keep alive message
	}
	if msg.length > MAX_PEER_MSG {
		//log.Println("Peer:", addr, "MSG Too Long:", msg.length)
		return msg, os.NewError("Message size too large")
	}
	//log.Println("Msg body length:", msg.length)
	//var msgId [1]byte
	msgId := make([]byte, 1)
	n, err = io.ReadFull(wire.conn, msgId)
	if err != nil || n != 1 {
		return msg, os.NewError("Read message id " + err.String())
	}
	msg.msgId = msgId[0]
	if wire.down_limit != nil && msg.msgId == piece {
		limit := int(float64(msg.length)/float64(1000)+0.5)
		for i := 0; i < limit; i++ {
			<- wire.down_limit.C
		}
	}
	var message_body []byte
	//var piece_buf []byte
	if msg.msgId == piece {
		message_body = make([]byte, 8) // allocate mem to read the position of the piece
		//piece_buf = make([]byte, msg.length - 9) // allocate mem to read the piece
	} else {
		message_body = make([]byte, msg.length - 1) // allocate mem to read the message
	}
	n, err = io.ReadFull(wire.conn, message_body) // read the payload
	if err != nil || n != len(message_body) {
		return msg, os.NewError("Read message body " + err.String())
	}
	if msg.msgId == piece {
		n, err = io.ReadFull(wire.conn, piece_buf) // read the piece
		if err != nil || n != len(piece_buf) {
			return msg, os.NewError("Read piece data " + err.String())
		}
		// Send piece to Files to store it
		fileMsg := new(FileMsg)
		fileMsg.Id = writeat
		fileMsg.Response = make(chan *FileMsg)
		fileMsg.Index = int64(binary.BigEndian.Uint32(message_body[0:4]))
		fileMsg.Begin = int64(binary.BigEndian.Uint32(message_body[4:8]))
		fileMsg.Bytes = piece_buf
		wire.files <- fileMsg
		fileMsg = <- fileMsg.Response
	}
	//n += 4
	// Assign to the message struct
	//msg.msgId = message_body[0]
	msg.payLoad = message_body
	return
}

func (wire *Wire) WriteMsg(msg *message) (err os.Error) {
	defer wire.writer.Flush()
	var n int
	
	num := make([]byte, 4)
	
	if wire.conn == nil {
		return os.NewError("Invalid connection")
	}
	binary.BigEndian.PutUint32(num, msg.length)
	if n, err = wire.writer.Write(num); err != nil || n != 4 {
		return os.NewError("Error sending message length " + err.String())
	}
	if msg.length == 0 {
		return
	}
	//buffer := bytes.NewBuffer(msg_byte[0:4])
	if err = wire.writer.WriteByte(msg.msgId); err != nil {
		return os.NewError("Error sending msgId " + err.String())
	}
	if len(msg.payLoad) > 0 {
		if n, err = wire.writer.Write(msg.payLoad); err != nil || n != len(msg.payLoad) {
			return os.NewError("Error sending payLoad" + err.String())
		}
		if msg.msgId == piece {
			wire.writer.Flush()
			// Obtain an io.Reader from Files
			fileMsg := new(FileMsg)
			fileMsg.Id = readat
			fileMsg.Index = int64(binary.BigEndian.Uint32(msg.payLoad[0:4]))
			fileMsg.Begin = int64(binary.BigEndian.Uint32(msg.payLoad[4:8]))
			fileMsg.Length = int64(binary.BigEndian.Uint32(msg.payLoad[8:12]))
			fileMsg.Response = make(chan *FileMsg)
			wire.files <- fileMsg
			fileMsg = <- fileMsg.Response
			if !fileMsg.Ok {
				return os.NewError("Error requesting piece from Files " + err.String())
			}
			// Bandwidth restriction
			if wire.up_limit != nil && msg.msgId == piece {
				limit := int(float64(fileMsg.Length)/float64(1000)+0.5)
				for i := 0; i < limit; i++ {
					<- wire.up_limit.C
				}
			}
			// Copy piece to connection
			var n int64
			n, err = io.Copy(wire.conn, fileMsg.Reader)
			if err != nil || n != fileMsg.Length {
				return os.NewError("Erro writing piece " + err.String())
			}
		}
	}
	return
}

func (wire *Wire) Close() {
	//log.Println(wire.conn)
	wire.conn.Close()
}
