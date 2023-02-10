package main

import (
	"bufio"
	"go-torrent/torrent"
	"log"
	"math/rand"
	"os"
)

func main() {
	file, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatalln("open file error")
		return
	}
	defer func() {
		_ = file.Close()
	}()
	tf, err := torrent.ParseFile(bufio.NewReader(file))
	if err != nil {
		log.Fatalln("parse file error")
		return
	}
	var peerId [torrent.IDLEN]byte
	// 本地客户端的唯一标识，随机生成
	_, _ = rand.Read(peerId[:])
	// 找到所有下载地址
	peers := torrent.FindPeers(tf, peerId)
	if len(peers) == 0 {
		log.Fatalln("can not find peers")
		return
	}
	task := &torrent.TorrentTask{
		PeerId:   peerId,
		PeerList: peers,
		InfoSHA:  tf.InfoSHA,
		FileName: tf.FileName,
		FileLen:  tf.FileLen,
		PieceLen: tf.PieceLen,
		PieceSHA: tf.PieceSHA,
	}
	_ = torrent.Download(task)
}
