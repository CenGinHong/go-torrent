package main

import (
	"bufio"
	"fmt"
	"go-torrent/torrent"
	"math/rand"
	"os"
)

func main() {
	file, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Println("open file error")
		return
	}
	defer func() {
		_ = file.Close()
	}()
	tf, err := torrent.ParseFile(bufio.NewReader(file))
	if err != nil {
		fmt.Println("parse file error")
		return
	}
	var peerId [torrent.IDLEN]byte
	// 本地客户端的唯一标识，一般回包含版本信息
	_, _ = rand.Read(peerId[:])
	// 找到所有下载地址
	peers := torrent.FindPeers(tf, peerId)
	if len(peers) == 0 {
		fmt.Println("can not find peers")
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
