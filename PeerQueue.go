// Peer write queue
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package main

import(
	"os"
	"bytes"
	//"log"
	)

type PeerQueue struct {
	phead, ptail, mhead, mtail, pn, mn int
	pieces map[int] *message
	messages map[int] *message
	length int
	in, delete, out chan *message
	//log *logger
}

func NewQueue(in, out, delete chan *message) (q *PeerQueue) {
	q = new(PeerQueue)
	q.mhead, q.mtail, q.phead, q.ptail, q.pn, q.mn = 0, 0, 0, 0, 0, 0
	q.pieces = make(map[int] *message, MAX_MSG_BUFFER)
	q.messages = make(map[int] *message, MAX_PIECE_BUFFER)
	q.in = in
	q.out = out
	q.delete = delete
	//q.log = l
	return
}

func (q *PeerQueue) Empty() bool {
	return (q.phead == q.ptail) && (q.mhead == q.mtail)
}

func (q *PeerQueue) Flush() {
	for key, _ := range(q.pieces) {
		q.pieces[key] = nil, false
	}
	for key, _ := range(q.messages) {
		q.messages[key] = nil, false
	}
	q.pieces = nil
	q.messages = nil
}

func (q *PeerQueue) Push(m *message) {
	if m.msgId == piece {
		if q.pn >= MAX_PIECE_BUFFER { return }
		q.pieces[q.phead] = m
		q.phead++
		q.pn++
	} else {
		if q.mn >= MAX_MSG_BUFFER { return }
		q.messages[q.mhead] = m
		q.mhead++
		q.mn++
	}
}

func (q *PeerQueue) Remove(m *message) {
	if m.msgId == cancel {
		key, err := q.SearchPiece(m)
		if err == nil { // Piece found
			// Delete this piece & reorder queue
			q.remove(key)
		}
		return
	}
}

func (q *PeerQueue) remove(key int) {
	for ;key > q.ptail; key-- {
		q.pieces[key] = q.pieces[key-1]
	}
	q.pieces[q.ptail] = nil, false
	q.ptail++
	q.pn--
	return
}

func (q *PeerQueue) TryPop() (m *message) {
	if q.mhead != q.mtail {
		m = q.messages[q.mtail]
	} else {
		m = q.pieces[q.ptail]
	}
	return
}

func (q *PeerQueue) Pop() {
	if q.mhead != q.mtail {
		q.messages[q.mtail] = nil, false
		q.mtail++
		q.mn--
	} else {
		q.pieces[q.ptail] = nil, false
		q.ptail++
		q.pn--
	}
}

func (q *PeerQueue) SearchPiece(m *message) (key int, err os.Error) {
	for key, msg := range(q.pieces) {
		if bytes.Equal(msg.payLoad[0:8], m.payLoad[0:8]) {
			return key, err
		}
	}
	return key, os.NewError("Piece not found")
}

func (q *PeerQueue) Run() {
	for !closed(q.in) && !closed(q.delete) && !closed(q.out) {
		//q.log.Output("PeerQueue -> Loop start")
		if q.Empty() {
			//q.log.Output("PeerQueue -> Queue empty, waiting for messages")
			select {
			case m := <- q.in:
				//q.log.Output("PeerQueue -> Received incoming message")
				if m == nil {
					goto exit
				}
				q.Push(m)
				//q.log.Output("PeerQueue -> Finished adding message")
			}
		} else {
			//q.log.Output("PeerQueue -> Queue not empty")
			select {
			case m := <- q.delete:
				//q.log.Output("PeerQueue -> Deleting message from queue")
				if m == nil {
					goto exit
				}
				q.Remove(m)
				//q.log.Output("PeerQueue -> Finished deleting message from queue")
			case m := <- q.in:
				//q.log.Output("PeerQueue -> Received new message, adding to queue")
				if m == nil {
					goto exit
				}
				q.Push(m)
				//q.log.Output("PeerQueue -> Finished adding new message to queue")
			case q.out <- q.TryPop():
				//q.log.Output("PeerQueue -> Popping message from queue")
				q.Pop()
				//q.log.Output("PeerQueue -> Finished popping message from queue")
			}
		}
		//q.log.Output("PeerQueue -> Pieces, phead:", q.phead, "ptail:", q.ptail, "pieces:", q.pieces)
		//q.log.Output("PeerQueue -> Messages, mhead:", q.mhead, "mtail:", q.mtail, "messages:", q.messages)
	}
exit:
	//q.log.Output("PeerQueue -> Flushing queue")
	q.Flush()
	close(q.out)
	//q.log.Output("PeerQueue -> Finished flushing queue")
}
