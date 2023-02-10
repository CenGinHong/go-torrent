package torrent

import (
	"bufio"
	"crypto/rand"
	"log"
	"os"
	"testing"
)

func TestTracker(t *testing.T) {
	file, _ := os.Open("../testfile/debian-iso.torrent")
	tf, _ := ParseFile(bufio.NewReader(file))
	var peerId [IDLEN]byte
	_, _ = rand.Read(peerId[:])
	peers := FindPeers(tf, peerId)
	for i, p := range peers {
		log.Printf("Peer %d, Ip: %s, Port: %d\n", i+1, p.IP, p.Port)
	}
}
