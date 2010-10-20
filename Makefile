include $(GOROOT)/src/Make.inc

all : wgo

TARG=wgo
DEPS=bencode wgo_io

GOFILES=\
	const.go \
	Torrent.go \
	Tracker.go \
	TrackerMgr.go \
	Files.go \
	Wire.go \
	Bitfield.go \
	Peer.go \
	PeerMgr.go \
	PieceMgr.go \
	PeerQueue.go \
	PieceData.go \
	ChokeMgr.go \
	Limiter.go \
	Stats.go \
	Listener.go \
	logger.go \
	test.go \

include $(GOROOT)/src/Make.cmd