wgo - Simple BitTorrent client in Go
==========

Roger Pau Monné - 2010

Introduction
------------

This project is based on the previous work of jackpal, Taipei-Torrent:
http://github.com/jackpal/Taipei-Torrent

Since Go is (or should become) a easy to use concurrent system programming
language I've decided to use it to develop a simple BitTorrent client. Some
of the functions are from the Taipei-Torrent project, and others are from
the gobit implementation found here:
http://github.com/jessta/gobit

Installation
------------

Simply run:

	./make.all

Usage
-----

wgo is still in a VERY early phase, it can only print basic information
about a torrent file, get peers fro mthe tracker an create the file
structure in the desired folder

	./wgo -torrent="path.to.torrent" -folder="/where/to/create/files"

Source code Hierarchy
---------------------

Since this client aims to make heavy use of concurrency, the approach I've
taken is the same as Haskell-Torrent, this is just a copy of the module
description made by Jesper Louis Andersen

   - **Process**: Process definitions for the different processes comprising Haskell Torrent
      - **ChokeMgr**: Manages choking and unchoking of peers, based upon the current speed of the peer
        and its current state. Global for multiple torrents.
      - **Console**: Simple console process. Only responds to 'quit' at the moment.
      - **Files**: Process managing the file system.
      - **Listen**: Not used at the moment. Step towards listening sockets.
      - **Peer**: Several process definitions for handling peers. Two for sending, one for receiving
        and one for controlling the peer and handle the state.
      - **PeerMgr**: Management of a set of peers for a single torrent.
      - **PieceMgr**: Keeps track of what pieces have been downloaded and what are missing. Also hands
        out blocks for downloading to the peers.
      - **Status**: Keeps track of uploaded/downloaded/left bytes for a single torrent. Could be globalized.
      - **Timer**: Timer events.
      - **Tracker**: Communication with the tracker.

   - **Protocol**: Modules for interacting with the various bittorrent protocols.
      - **Wire**: The protocol used for communication between peers.

   - **Top Level**:
      - **Const**: Several fine-tunning options, untill we are able to read them from a configuration file
      - **Torrent**: Various helpers and types for Torrents.
      - **Test**: Currently it holds the main part of the program

There's a nice graph that shows the proccess comunications:
	http://jlouisramblings.blogspot.com/2010/01/thoughts-on-process-hierarchies-in.html
	http://jlouisramblings.blogspot.com/2009/12/concurrency-bittorrent-clients-and.html
	http://jlouisramblings.blogspot.com/2009/12/on-peer-processes.html


