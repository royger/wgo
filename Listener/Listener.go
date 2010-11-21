package listener

import(
	"net"
	"log"
	"os"
	"wgo/peers"
	"strings"
)

type Listener struct {
	listener net.Listener
	peerMgr peers.PeerMgr
}

func NewListener(ip, port string, peerMgr peers.PeerMgr) (l *Listener, cport string, err os.Error) {
	l = new(Listener)
	l.listener, err = net.Listen("tcp4", ip + ":" + port)
	if err != nil {
		log.Println(err)
		//return
	}
	l.peerMgr = peerMgr
	log.Println("Listening on:", l.listener.Addr().String())
	cport = l.listener.Addr().String()[strings.LastIndex(l.listener.Addr().String(), ":")+1:]
	go l.Run()
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
		l.peerMgr.AddPeer(c)
	}
}
