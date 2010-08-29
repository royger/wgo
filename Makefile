include $(GOROOT)/src/Make.inc

all : wgo

TARG=wgo

GOFILES=\
	const.go \
	Torrent.go \
	Tracker.go \
	Files.go \
	Wire.go \
	Bitfield.go \
	Peer.go \
	PeerMgr.go \
	PieceMgr.go \
	PeerQueue.go \
	PieceData.go \
	Stats.go \
	Listener.go \
	logger.go \
	test.go \

include $(GOROOT)/src/Make.cmd
