package torrent

import (
	"crypto/rand"
	"encoding/binary"
	"go-torrent/bencode"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	PeerPort int = 6666
	IpLen    int = 4
	PortLen  int = 2
	PeerLen      = IpLen + PortLen
)

const IDLEN int = 20

type PeerInfo struct {
	IP   net.IP
	Port uint16
}

type TrackerResp struct {
	Interval int    `bencode:"interval"`
	Peers    string `bencode:"peers"`
}

func buildUrl(tf *TorrentFile, peerId [IDLEN]byte) (string, error) {
	if _, err := rand.Read(peerId[:]); err != nil {
		return "", err
	}
	// 转换成url
	base, err := url.Parse(tf.Announce)
	if err != nil {
		log.Printf("Announce Error: %s\n", tf.Announce)
		return "", err
	}
	// 生成参数
	params := url.Values{
		// 文件标识
		"info_hash": []string{string(tf.InfoSHA[:])},
		// 下载器标识
		"peer_id": []string{string(peerId[:])},
		// 端口
		"port": []string{strconv.Itoa(PeerPort)},
		// 暂时无用，用默认值
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"compact":    []string{"1"},
		// 剩余多少，设为初始值
		"left": []string{strconv.Itoa(tf.FileLen)},
	}
	base.RawQuery = params.Encode()
	return base.String(), nil
}

// FindPeers 找peer的下载地址
func FindPeers(tf *TorrentFile, peerId [IDLEN]byte) []PeerInfo {
	u, err := buildUrl(tf, peerId)
	if err != nil {
		log.Printf("Build Tracker Url Error: %v\n", err)
		return nil
	}
	cli := &http.Client{Timeout: 15 * time.Second}
	// 发送一个http get
	resp, err := cli.Get(u)
	if err != nil {
		log.Printf("Fail to Connect to Tracker: %v\n", err)
		return nil
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	trackResp := new(TrackerResp)
	// 是bencode格式，需要unmarshal
	if err = bencode.Unmarshal(resp.Body, trackResp); err != nil {
		log.Printf("Tracker Response Error: %v\n", err)
		return nil
	}

	return buildPeerInfo([]byte(trackResp.Peers))
}

// 将紧凑排列的信息展开
func buildPeerInfo(peers []byte) []PeerInfo {
	// 总长/ 每一位peer的信息位数
	cnt := len(peers) / PeerLen
	// 不能整除就有bug
	if len(peers)%PeerLen != 0 {
		log.Printf("Received malformed peers")
		return nil
	}
	infos := make([]PeerInfo, cnt)
	for i := 0; i < cnt; i++ {
		offset := i * PeerLen
		infos[i].IP = peers[offset : offset+IpLen]
		// 大小端转换
		infos[i].Port = binary.BigEndian.Uint16(peers[offset+IpLen : offset+PeerLen])
	}
	return infos
}
