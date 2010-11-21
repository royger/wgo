package main

import (
	"bytes"
	"crypto/sha1"
	"io"
	//"io/ioutil"
	"wgo/bencode"
	//"log"
	"http"
	"os"
	"strings"
	"container/vector"
)

func getString(m map[string]interface{}, k string) string {
	if v, ok := m[k]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getArrayString(m map[string]interface{}, k string) (list []string) {
	list = make([]string, 0)
	if v, ok := m[k]; ok {
		if f, ok := v.(vector.Vector); ok {
			for _, s := range f {
				if l, ok := s.(vector.Vector); ok {
					for _, q := range l {
						if e, ok := q.(string); ok {
							list = append(list, e)
						}
					}
				}
			}
		}
	}
	return
}

func NewTorrent(torrent string) (metaInfo *bencode.MetaInfo, err os.Error) {
	var input io.ReadCloser
	if strings.HasPrefix(torrent, "http:") {
		// 6g compiler bug prevents us from writing r, _, err :=
		var r *http.Response
		if r, _, err = http.Get(torrent); err != nil {
			return
		}
		input = r.Body
	} else {
		if input, err = os.Open(torrent, os.O_RDONLY, 0666); err != nil {
			return
		}
	}

	// We need to calcuate the sha1 of the Info map, including every value in the
	// map. The easiest way to do this is to read the data using the Decode
	// API, and then pick through it manually.
	var m interface{}
	m, err = bencode.Decode(input)
	input.Close()
	if err != nil {
		err = os.NewError("Couldn't parse torrent file phase 1: " + err.String())
		return
	}

	topMap, ok := m.(map[string]interface{})
	if !ok {
		err = os.NewError("Couldn't parse torrent file phase 2.")
		return
	}

	infoMap, ok := topMap["info"]
	if !ok {
		err = os.NewError("Couldn't parse torrent file. info")
		return
	}
	var b bytes.Buffer
	if err = bencode.Marshal(&b, infoMap); err != nil {
		return
	}
	hash := sha1.New()
	hash.Write(b.Bytes())

	var m2 bencode.MetaInfo
	err = bencode.Unmarshal(&b, &m2.Info)
	if err != nil {
		return
	}
	//log.Println(m2.Info)
	m2.Infohash = string(hash.Sum())
	m2.Announce = getString(topMap, "announce")
	m2.CreationDate = getString(topMap, "creation date")
	m2.Comment = getString(topMap, "comment")
	m2.CreatedBy = getString(topMap, "created by")
	m2.Encoding = getString(topMap, "encoding")
	m2.Announce_list = append(getArrayString(topMap, "announce-list"), m2.Announce)

	metaInfo = &m2
	return
}
