// File for constants
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package main

const(
	CLIENT_ID = "-wg0001"
	FILE_PERM = 0666
	FOLDER_PERM = 0755
	PROTOCOL = "BitTorrent protocol"
	MAX_PEER_MSG = 130*1024
	ACTIVE_PEERS = 10
	INACTIVE_PEERS = 40
	TRACKER_ERR_INTERVAL = 20*NS_PER_S
	NS_PER_S = 1000000000
	KEEP_ALIVE_MSG = 120*NS_PER_S
	TIMEOUT = 6*60*NS_PER_S
	STANDARD_BLOCK_LENGTH = 16 * 1024
	MAX_PIECE_LENGTH = 128*1024
	NUM_PEERS = 100
	MAX_REQUESTS = 2
	CLEAN_REQUESTS = 30
	MAX_MSG_BUFFER = 20
	MAX_PIECE_BUFFER = 10
	HASHERS = 10
	)

const (
	choke = iota
	unchoke
	interested
	uninterested
	have
	bitfield
	request
	piece
	cancel
	port
	exit
	our_request
)
