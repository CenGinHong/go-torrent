package torrent

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

type MsgId uint8

const (
	MsgChoke MsgId = iota // chock是默认不提供上传功能
	MsgUnchoke
	MsgInterested  // 想要下载某一个piece
	MsgNotInterest // 不下载，只做种上传
	MsgHave        // 请求过，同时对端也在下载，例如先告诉你有123，然后他又下完4了，就告诉你我有4来更新位图
	MsgBitfield
	MsgRequest // 下载请求，下载那个pieces,start，length
	MsgPiece   // 对端回应pieces的byte
	MsgCancel  // 取消
	// 前四个是上传相关的
	// 接着两个是数据情况
	// 剩下是数据传输类
)

type PeerMsg struct {
	Id      MsgId
	Payload []byte // 视乎上面id的内容决定内容
}

type PeerConn struct {
	net.Conn
	Choked  bool
	Field   Bitfield
	peer    PeerInfo
	peerId  [IDLEN]byte
	infoSHA [SHALEN]byte
}

func handshake(conn net.Conn, infoSHA [SHALEN]byte, peerId [IDLEN]byte) error {
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return err
	}
	// 生成握手消息，68Bytes?
	req := NewHandshakeMsg(infoSHA, peerId)
	_, err := WriteHandshake(conn, req)
	if err != nil {
		fmt.Println("send handshake failed")
		return err
	}
	// 读出来返回的handShakeMag
	res, err := ReadHandshake(conn)
	if err != nil {
		fmt.Println("read handshake failed")
		return err
	}
	// 校验 HandshakeMsg
	if !bytes.Equal(res.InfoSHA[:], infoSHA[:]) {
		fmt.Println("check handshake failed")
		return fmt.Errorf("handshake msg error： " + string(res.InfoSHA[:]))
	}
	return nil
}

// NewConn 将client 和 peer之间的conn抽象成一个PeerConn
func NewConn(peer PeerInfo, infoSHA [SHALEN]byte, peerId [IDLEN]byte) (*PeerConn, error) {
	// 连在一起
	addr := net.JoinHostPort(peer.IP.String(), strconv.Itoa(int(peer.Port)))
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		fmt.Println("set tcp conn failed: " + addr)
		return nil, err
	}
	if err = handshake(conn, infoSHA, peerId); err != nil {
		return nil, err
	}
	c := &PeerConn{
		Conn:    conn,
		Choked:  true,
		peer:    peer,
		peerId:  peerId,
		infoSHA: infoSHA,
	}
	// 发送一个peerMsg，获取对端的bitmap,记录到peerConn的字段
	if err = fillBitfield(c); err != nil {
		fmt.Println("fill bitfield failed")
		return nil, err
	}
	return c, nil

}

func fillBitfield(c *PeerConn) error {
	if err := c.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}

	msg, err := c.ReadMsg()
	if err != nil {
		return err
	}
	if msg == nil {
		return fmt.Errorf("expected bitfield")
	}

	if msg.Id != MsgBitfield {
		return fmt.Errorf("expected bitfield, get " + strconv.Itoa(int(msg.Id)))
	}
	fmt.Println("fill bitfield: " + c.peer.IP.String())
	// 将位图拷进去
	c.Field = msg.Payload
	return nil
}

func (c *PeerConn) ReadMsg() (*PeerMsg, error) {
	// 读 msg 的长度
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(c, lenBuf); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lenBuf)
	if length == 0 {
		return nil, nil
	}
	msgBuf := make([]byte, length)
	if _, err := io.ReadFull(c, msgBuf); err != nil {
		return nil, err
	}
	return &PeerMsg{
		Id:      MsgId(msgBuf[0]),
		Payload: msgBuf[1:],
	}, nil
}
