// Reads info from torrent file and saves it in the appropiate structs.
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package main

import(
	"os"
	//"bytes"
	//"crypto/sha1"
	"strings"
	"http"
	"io"
	"wgo/bencode"
	)

// Struct to save torrent info

/*type Torrent struct {
	Info         InfoDict
	InfoHash     string
	Announce     string
	CreationDate string "creation date"
	Comment      string
	CreatedBy    string "created by"
	Encoding     string
}*/

// Struct to save info about torrent files

/*type InfoDict struct {
	PieceLength int64 "piece length"
	Pieces      string
	Private     int64
	Name        string
	// Single File Mode
	Length int64
	Md5sum string
	// Multiple File mode
	Files []FileDict
}

type FileDict struct {
	Length int64
	Path   []string
	Md5sum string
}

// Use to get the values from the bencode interface
func getString(m map[string]interface{}, k string) string {
	if v, ok := m[k]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}*/

func NewTorrent(file string) (torrent *bencode.Torrent, err os.Error) {
	//torrent = new(Torrent)
	var input io.ReadCloser
	
	if strings.HasPrefix(file, "http:") {
		// 6g compiler bug prevents us from writing r, _, err :=
		var r *http.Response
		r, _, err = http.Get(file)
		input = r.Body
	} else {
		input, err = os.Open(file, os.O_RDONLY, 0666)
	}
	if err != nil {
		return
	}
	
	defer input.Close()
	torrent, err = bencode.Parse(input)
	// We need to calcuate the sha1 of the Info map, including every value in the
	// map. The easiest way to do this is to read the data using the Decode
	// API, and then pick through it manually.
	/*var m interface{}
	
	m, err = bencode.Decode(input)
	if err != nil {
		err = os.NewError("Couldn't parse torrent file.")
		return
	}

	topMap, ok := m.(map[string]interface{})
	if !ok {
		err = os.NewError("Couldn't parse torrent file.")
		return
	}

	infoMap, ok := topMap["info"]
	if !ok {
		err = os.NewError("Couldn't parse torrent file. info")
		return
	}
	var b bytes.Buffer
	err = bencode.Marshal(&b, infoMap)
	if err != nil {
		return
	}
	hash := sha1.New()
	hash.Write(b.Bytes())

	err = bencode.Unmarshal(&b, &torrent.Info)
	if err != nil {
		return
	}
	
	torrent.InfoHash = string(hash.Sum())
	torrent.Announce = getString(topMap, "announce")
	torrent.CreationDate = getString(topMap, "creation date")
	torrent.Comment = getString(topMap, "comment")
	torrent.CreatedBy = getString(topMap, "created by")
	torrent.Encoding = getString(topMap, "encoding")*/
	
	
	
	return
}
