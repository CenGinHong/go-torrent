package torrent

import (
	"bytes"
	"crypto/sha1"
	"go-torrent/bencode"
	"io"
	"log"
)

type rawInfo struct {
	Length      int    `bencode:"length"`
	Name        string `bencode:"name"`
	PieceLength int    `bencode:"piece length"` // 对应的值是文件以字节为单位的每个分片的长度
	Pieces      string `bencode:"pieces"`       // 将字节序列按 20 个字节为一组切分开, 则每组都是文件相对应 piece 的 SHA1 哈希值
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
	raw := &rawFile{}
	err := bencode.Unmarshal(r, raw)
	if err != nil {
		log.Println("Fail to parse torrent file")
		return nil, err
	}
	ret := &TorrentFile{
		Announce: raw.Announce,
		FileName: raw.Info.Name,
		FileLen:  raw.Info.Length,
		PieceLen: raw.Info.PieceLength,
	}

	// 计算 info SHA
	buf := new(bytes.Buffer)
	// Marshal是不对的，应该要根据tag去做marshal
	wlen := bencode.Marshal(buf, raw.Info)
	if wlen == 0 {
		log.Fatal("raw file info error")
	}
	ret.InfoSHA = sha1.Sum(buf.Bytes())

	// 计算 pieces SHA
	// pieces在文件中读到
	bys := []byte(raw.Info.Pieces)
	cnt := len(bys) / SHALEN
	hashes := make([][SHALEN]byte, cnt)
	for i := 0; i < cnt; i++ {
		copy(hashes[i][:], bys[i*SHALEN:(i+1)*SHALEN])
	}
	ret.PieceSHA = hashes
	return ret, nil
}
