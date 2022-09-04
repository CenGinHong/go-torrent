package torrent

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"go-torrent/bencode"
	"io"
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
	PeerLen  int = IpLen + PortLen
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

func buildUrl(tf *TorrentFile) (string, error) {
	// 本地客户端的唯一标识，一般回包含版本信息
	var peerId [20]byte
	if _, err := rand.Read(peerId[:]); err != nil {
		return "", err
	}
	// 转换成url
	base, err := url.Parse(tf.Announce)
	if err != nil {
		fmt.Println("Announce Error: " + tf.Announce)
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

func FindPeers(tf *TorrentFile) []PeerInfo {
	u, err := buildUrl(tf)
	if err != nil {
		fmt.Println("Build Tracker Url Error: " + err.Error())
		return nil
	}
	cli := &http.Client{Timeout: 15 * time.Second}
	// 发送一个http get
	resp, err := cli.Get(u)
	if err != nil {
		fmt.Println("Fail to Connect to Tracker: " + err.Error())
		return nil
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	trackResp := new(TrackerResp)
	// 是bencode格式，需要unmarshal
	err = bencode.Unmarshal(resp.Body, trackResp)
	if err != nil {
		fmt.Println("Tracker Response Error" + err.Error())
		return nil
	}

	return buildPeerInfo([]byte(trackResp.Peers))
}

// 将紧凑排列的信息展开
func buildPeerInfo(peers []byte) []PeerInfo {
	cnt := len(peers) / PeerLen
	if len(peers)%PeerLen != 0 {
		fmt.Println("Received malformed peers")
		return nil
	}
	infos := make([]PeerInfo, cnt)
	for i := 0; i < cnt; i++ {
		offset := i * PeerLen
		infos[i].IP = net.IP(peers[offset : offset+IpLen])
		infos[i].Port = binary.BigEndian.Uint16(peers[offset+IpLen : offset+PeerLen])
	}
	return infos
}
