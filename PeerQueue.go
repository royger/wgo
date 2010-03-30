// Peer write queue
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package main

import(
	"os"
	"bytes"
	)

type PeerQueue struct {
	phead, ptail, mhead, mtail int
	pieces map[int] message
	messages map[int] message
	length int
	in, delete, out chan message
}

func NewQueue(in, out, delete chan message) (q *PeerQueue) {
	q = new(PeerQueue)
	q.mhead, q.mtail, q.phead, q.ptail = 0, 0, 0, 0
	q.pieces = make(map[int] message)
	q.messages = make(map[int] message)
	q.in = in
	q.out = out
	q.delete = delete
	return
}

func (q *PeerQueue) Empty() bool {
	return (q.phead == q.ptail) && (q.mhead == q.mtail)
}

func (q *PeerQueue) Push(m message) {
	if m.msgId == piece {
		q.pieces[q.phead] = m
		q.phead++
	} else {
		q.messages[q.mhead] = m
		q.mhead++
	}
}

func (q *PeerQueue) Remove(m message) {
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
	q.pieces[q.ptail] = message{}, false
	q.ptail++
	return
}

func (q *PeerQueue) Pop() (m message) {
	if q.mhead != q.mtail {
		m = q.messages[q.mtail]
		q.messages[q.mtail] = m, false
		q.mtail++
	} else {
		m = q.pieces[q.ptail]
		q.pieces[q.ptail] = m, false
		q.ptail++
	}
	return
}

func (q *PeerQueue) SearchPiece(m message) (key int, err os.Error) {
	for key, msg := range(q.pieces) {
		if bytes.Equal(msg.payLoad[0:8], m.payLoad[0:8]) {
			return key, err
		}
	}
	return key, os.NewError("Piece not found")
}

func (q *PeerQueue) Run() {
	for !closed(q.in) {
		if q.Empty() {
			select {
			case m := <-q.in:
				q.Push(m)
			}
		} else {
			select {
			case m := <- q.delete:
				q.Remove(m)
			case m := <- q.in:
				q.Push(m)
			case q.out <- q.Pop():
				continue
			}
		}
	}
	close(q.out)
}
