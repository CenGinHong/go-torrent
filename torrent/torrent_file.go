package torrent

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"go-torrent/bencode"
	"io"
)

type rawInfo struct {
	Length      int    `bencode:"length"`
	Name        string `bencode:"name"`
	PieceLength int    `bencode:"piece length"`
	Pieces      string `bencode:"prices"`
}

type rawFile struct {
	Announce string  `bencode:"announce"`
	Info     rawInfo `bencode:"info"`
}

const SHALEN int = 20

type TorrentFile struct {
	Announce string       // tracker的url
	InfoSHA  [SHALEN]byte // 需要下载文件的唯一标识
	FileName string       // 本地文件的文件名
	FileLen  int          // 文件长度
	PieceLen int
	PieceSHA [][SHALEN]byte // 文件校验使用
}

func ParseFile(r io.Reader) (*TorrentFile, error) {
	raw := new(rawFile)
	err := bencode.Unmarshal(r, raw)
	if err != nil {
		fmt.Println("Fail to parse torrent file")
		return nil, err
	}
	ret := new(TorrentFile)
	ret.Announce = raw.Announce
	ret.FileName = raw.Info.Name
	ret.FileLen = raw.Info.Length
	ret.PieceLen = raw.Info.PieceLength

	// 计算 info SHA
	buf := new(bytes.Buffer)
	// Marshal是不对的，应该要根据tag去做marshal
	wlen := bencode.Marshal(buf, raw.Info)
	if wlen == 0 {
		fmt.Println("raw file info error")
	}
	ret.InfoSHA = sha1.Sum(buf.Bytes())

	// 计算 pieces SHA
	bys := []byte(raw.Info.Pieces)
	cnt := len(bys) / SHALEN
	// 看来pieces应该是文件直接得到的
	hashes := make([][SHALEN]byte, cnt)
	for i := 0; i < cnt; i++ {
		copy(hashes[i][:], bys[i*SHALEN:(i+1)*SHALEN])
	}
	ret.PieceSHA = hashes
	return ret, nil
}
