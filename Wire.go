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
	)

type Wire struct {
	pstrlen uint8
	pstr string
	reserved []byte
	infohash []byte
	peerid	[]byte
	conn net.Conn
}
	
type message struct {
	length	uint32;
	msgId	uint8;
	payLoad	[]byte;
	peer	string
}

func NewWire(infohash, peerid string, conn net.Conn) (wire *Wire) {
	wire = new(Wire)
	wire.pstr = PROTOCOL
	wire.pstrlen = (uint8)(len(wire.pstr))
	wire.reserved = make([]byte,8)
	wire.infohash = []byte(infohash)
	wire.peerid = []byte(peerid)
	wire.conn = conn
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
	// Reading peer handshake
	var header [68]byte
	_, err = wire.conn.Read(header[0:1])
	if err != nil {
		return peerid, os.NewError("Reading handshake length: " + err.String())
	}
	if header[0] != 19 {
		return peerid, os.NewError("Invalid length")
	}
	_, err = wire.conn.Read(header[1:20])
	if err != nil {
		return peerid, os.NewError("Reading protocol string: " + err.String())
	}
	if string(header[1:20]) != "BitTorrent protocol" {
		return peerid, os.NewError("Unknown protocol")
	}
	// Read rest of header
	_, err = wire.conn.Read(header[20:])
	if err != nil {
		return peerid, os.NewError("Reading payload of the handshake: " + err.String())
	}
	// See if infohash matches
	if !bytes.Equal(header[28:48], wire.infohash) {
		return peerid, os.NewError("InfoHash doesn't match")
	}
	peerid = string(header[48:68])
	return 
}

func (wire *Wire) ReadMsg() (msg *message, err os.Error) {
	msg = new(message)
	msg.peer = wire.conn.RemoteAddr().String()
	var length_header [4]byte
	_, err = wire.conn.Read(length_header[0:]) // read msg length
	if err != nil {
		return msg, os.NewError("Read header length " + err.String())
	}
	msg.length = binary.BigEndian.Uint32(length_header[0:]) // Convert length
	if msg.length == 0 {
		return // Keep alive message
	}
	if msg.length > MAX_PEER_MSG {
		return msg, os.NewError("Message size too large")
	}
	//log.Stderr("Msg body length:", msg.length)
	message_body := make([]byte, msg.length) // allocate mem to read the rest of the message
	_, err = wire.conn.Read(message_body[0:]) // read the rest
	if err != nil {
		return msg, os.NewError("Read message body " + err.String())
	}
	// Assign to the message struct
	msg.msgId = message_body[0]
	msg.payLoad = bytes.Add(msg.payLoad, message_body[1:])
	return
}

func (wire *Wire) WriteMsg(msg message) (err os.Error) {
	msg_byte := make([]byte, 4 + msg.length)
	binary.BigEndian.PutUint32(msg_byte[0:4], msg.length)
	if msg.length == 0 {  // Keep-alive message
		_, err = wire.conn.Write(msg_byte)
		return
	}
	buffer := bytes.NewBuffer(msg_byte[0:4])
	buffer.WriteByte(msg.msgId) // Write msg
	if len(msg.payLoad) > 0 {
		buffer.Write(msg.payLoad)
	}
	_, err = wire.conn.Write(buffer.Bytes())
	return
}

