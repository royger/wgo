// File for constants
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package main

const(
	FILE_PERM = 0666
	FOLDER_PERM = 0755
	PROTOCOL = "BitTorrent protocol"
	MAX_PEER_MSG = 130*1024
	)

const (
	choke	= iota;
	unchoke;
	interested;
	uninterested;
	have;
	bitfield;
	request;
	piece;
	cancel;
	port;
)
