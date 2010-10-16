package main

import(
	"net"
	"log"
	"os"
)

type Listener struct {
	listener net.Listener
	outPeerMgr chan net.Conn
}

func NewListener(ip, port string, outPeerMgr chan net.Conn) (l *Listener, err os.Error) {
	l = new(Listener)
	l.listener, err = net.Listen("tcp4", ip + ":" + port)
	if err != nil {
		log.Println(err)
		//return
	}
	l.outPeerMgr = outPeerMgr
	log.Println("Listening on:", l.listener.Addr().String())
	return
}

func (l *Listener) Run() {
	for {
		c, err := l.listener.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		//log.Println("Listener -> New connection from:", c.RemoteAddr().String())
		l.outPeerMgr <- c
	}
}
