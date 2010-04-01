include $(GOROOT)/src/Make.$(GOARCH)

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
	test.go \

include $(GOROOT)/src/Make.cmd
