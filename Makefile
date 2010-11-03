include $(GOROOT)/src/Make.inc

all : clean wgo

TARG=wgo
DEPS=Bitfield bencode wgo_io Stats Files Limiter Peers Choke Listener Tracker

GOFILES=\
	const.go \
	Torrent.go \
	logger.go \
	test.go \

include $(GOROOT)/src/Make.cmd

clean:
	rm -rf *.o *.a *.[$(OS)] [$(OS)].out $(CLEANFILES)
	for i in $(DEPS); do $(MAKE) -C $$i clean; done