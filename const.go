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
	ACTIVE_PEERS = 45
	UNUSED_PEERS = 200
	INCOMING_PEERS = 10
	PERCENT_UNUSED_PEERS = 20
	TRACKER_ERR_INTERVAL = 20*NS_PER_S
	NS_PER_S = 1000000000
	KEEP_ALIVE_MSG = 120*NS_PER_S
	KEEP_ALIVE_RESP = 240*NS_PER_S
	TIMEOUT = 6*60*NS_PER_S
	STANDARD_BLOCK_LENGTH = 16 * 1024
	MAX_PIECE_LENGTH = 128*1024
	NUM_PEERS = 100
	MAX_REQUESTS = 2048
	DEFAULT_REQUESTS = 20
	CLEAN_REQUESTS = 120
	HASHERS = 5
	TRACKER_UPDATE = 60
	PONDERATION_TIME = 10 // in seconds
	CHOKE_ROUND = 10
	OPTIMISTIC_UNCHOKE = 30
	UPLOADING_PEERS = 5
	SNUBBED_PERIOD = 60
	REQUESTS_LENGTH = 10 // time of requests to ask to a peer (10s of pieces)
	MAX_PIECE_REQUESTS = 4
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
	flush
)
