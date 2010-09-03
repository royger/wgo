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
	//"log"
	"io"
	//"math"
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
}
	
type message struct {
	length	uint32
	msgId	uint8
	payLoad	[]byte
	addr	[]string
}

func NewWire(infohash, peerid string, conn net.Conn, up_limit, down_limit *time.Ticker) (wire *Wire, err os.Error) {
	wire = new(Wire)
	wire.pstr = PROTOCOL
	wire.pstrlen = (uint8)(len(wire.pstr))
	wire.reserved = make([]byte,8)
	wire.infohash = []byte(infohash)
	wire.peerid = []byte(peerid)
	wire.conn = conn
	err = wire.conn.SetTimeout(KEEP_ALIVE_RESP)
	wire.up_limit = up_limit
	wire.down_limit = down_limit
	return
}

func (wire *Wire) Handshake() (peerid string, err os.Error) {
	// Sending handshake
	msg := make([]byte, 49 + wire.pstrlen)
	buffer := bytes.NewBuffer(msg[0:0])
	buffer.WriteByte(wire.pstrlen)
	buffer.WriteString(wire.pstr)
	buffer.Write(wire.reserved)
	buffer.Write(wire.infohash)
	buffer.Write(wire.peerid)
	_, err = wire.conn.Write(buffer.Bytes())
	//log.Stderr("Writting header", buffer.Bytes())
	// Reading peer handshake
	var header [68]byte
	_, err = io.ReadFull(wire.conn, header[0:1])
	if err != nil {
		return peerid, os.NewError("Reading handshake length: " + err.String())
	}
	if header[0] != 19 {
		return peerid, os.NewError("Invalid length")
	}
	_, err = io.ReadFull(wire.conn, header[1:20])
	if err != nil {
		return peerid, os.NewError("Reading protocol string: " + err.String())
	}
	if string(header[1:20]) != "BitTorrent protocol" {
		return peerid, os.NewError("Unknown protocol")
	}
	// Read rest of header
	_, err = io.ReadFull(wire.conn, header[20:])
	if err != nil {
		return peerid, os.NewError("Reading payload of the handshake: " + err.String())
	}
	// See if infohash matches
	if !bytes.Equal(header[28:48], wire.infohash) {
		return peerid, os.NewError("InfoHash doesn't match")
	}
	peerid = string(header[48:68])
	//log.Stderr("Received header", header)
	return 
}

func (wire *Wire) ReadMsg() (msg *message, n int, err os.Error) {
	if wire.conn == nil {
		return msg, n, os.NewError("Invalid connection")
	}
	msg = new(message)
	addr := wire.conn.RemoteAddr()
	if addr == nil {
		return msg, n, os.NewError("Invalid address")
	}
	msg.addr = []string{addr.String()}
	var length_header [4]byte
	_, err = io.ReadFull(wire.conn, length_header[0:4]) // read msg length
	if err != nil {
		return msg, n, os.NewError("Read header length " + err.String())
	}
	msg.length = binary.BigEndian.Uint32(length_header[0:4]) // Convert length
	if msg.length == 0 {
		return // Keep alive message
	}
	if msg.length > MAX_PEER_MSG {
		//log.Stderr("Peer:", addr, "MSG Too Long:", msg.length)
		return msg, n, os.NewError("Message size too large")
	}
	//log.Stderr("Msg body length:", msg.length)
	message_body := make([]byte, msg.length) // allocate mem to read the rest of the message
	if wire.down_limit != nil && msg.length > 13 {
		limit := int(float64(msg.length)/float64(1000)+0.5)
		for i := 0; i < limit; i++ {
			<- wire.down_limit.C
		}
	}
	n, err = io.ReadFull(wire.conn, message_body) // read the rest
	if err != nil {
		return msg, n, os.NewError("Read message body " + err.String())
	}
	n += 4
	// Assign to the message struct
	msg.msgId = message_body[0]
	msg.payLoad = bytes.Add(msg.payLoad, message_body[1:])
	return
}

func (wire *Wire) WriteMsg(msg *message) (n int, err os.Error) {
	if wire.conn == nil {
		return n, os.NewError("Invalid connection")
	}
	msg_byte := make([]byte, 4 + msg.length)
	binary.BigEndian.PutUint32(msg_byte[0:4], msg.length)
	if msg.length == 0 {  // Keep-alive message
		n, err = wire.conn.Write(msg_byte)
		return
	}
	buffer := bytes.NewBuffer(msg_byte[0:4])
	buffer.WriteByte(msg.msgId) // Write msg
	if len(msg.payLoad) > 0 {
		buffer.Write(msg.payLoad)
	}
	if wire.up_limit != nil && msg.msgId == piece {
		//log.Stderr("Wire -> Tokens:", int(math.Floor(float64(msg.length)/float64(1000))))
		limit := int(float64(msg.length)/float64(1000)+0.5)
		for i := 0; i < limit; i++ {
			<- wire.up_limit.C
		}
	}
	n, err = wire.conn.Write(buffer.Bytes())
	return
}

func (wire *Wire) Close() {
	//log.Stderr(wire.conn)
	wire.conn.Close()
}
