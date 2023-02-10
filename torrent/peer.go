package torrent

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
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
	Choked  bool // 不提供上传
	Field   Bitfield
	peer    PeerInfo
	peerId  [IDLEN]byte
	infoSHA [SHALEN]byte
}

// handshake 该过程进行了文件分片sha的校验
func handshake(conn net.Conn, infoSHA [SHALEN]byte, peerId [IDLEN]byte) error {
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return err
	}
	defer func() {
		_ = conn.SetDeadline(time.Time{})
	}()
	// 生成握手消息，68Bytes?
	req := NewHandshakeMsg(infoSHA, peerId)
	if _, err := WriteHandshake(conn, req); err != nil {
		log.Println("send handshake failed")
		return err
	}
	// 读出来返回的handShakeMag
	res, err := ReadHandshake(conn)
	if err != nil {
		log.Println("read handshake failed")
		return err
	}
	// 校验 HandshakeMsg
	if !bytes.Equal(res.InfoSHA[:], infoSHA[:]) {
		log.Println("check handshake failed")
		return fmt.Errorf("handshake msg error: %s", string(res.InfoSHA[:]))
	}
	return nil
}

// NewConn 将client 和 peer之间的conn抽象成一个PeerConn
func NewConn(peer PeerInfo, infoSHA [SHALEN]byte, peerId [IDLEN]byte) (*PeerConn, error) {
	// 连在一起
	addr := net.JoinHostPort(peer.IP.String(), strconv.Itoa(int(peer.Port)))
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		log.Printf("set tcp conn failed: %s\n", addr)
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
		log.Println("fill bitfield failed")
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
		return errors.New("expected bitfield")
	}
	if msg.Id != MsgBitfield {
		return fmt.Errorf("expected bitfield, get: %s", strconv.Itoa(int(msg.Id)))
	}
	log.Println("fill bitfield: " + c.peer.IP.String())
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

// 消息的前四个字节用放长度
const lenBytes uint32 = 4

func (c *PeerConn) WriteMsg(m *PeerMsg) (int, error) {
	var buf []byte
	if m == nil {
		return 0, fmt.Errorf("msg cannot be nil")
	}
	length := uint32(len(m.Payload) + 1)
	buf = make([]byte, lenBytes+length)
	// 使用大端模式写入
	binary.BigEndian.PutUint32(buf[0:lenBytes], length)
	// 写入消息类型
	buf[lenBytes] = byte(m.Id)
	copy(buf[lenBytes+1:], m.Payload)
	return c.Write(buf)
}

func NewRequestMsg(index, offset, length int) *PeerMsg {
	payload := make([]byte, 12)
	binary.BigEndian.PutUint32(payload[0:4], uint32(index))
	binary.BigEndian.PutUint32(payload[4:8], uint32(offset))
	binary.BigEndian.PutUint32(payload[8:12], uint32(length))
	return &PeerMsg{MsgRequest, payload}
}

func GetHaveIndex(msg *PeerMsg) (int, error) {
	if msg.Id != MsgHave {
		return 0, fmt.Errorf("expected MsgHave (Id %d), got Id %d", MsgHave, msg.Id)
	}
	if len(msg.Payload) != 4 {
		return 0, fmt.Errorf("expected payload length 4, got length %d", len(msg.Payload))
	}
	index := int(binary.BigEndian.Uint32(msg.Payload))
	return index, nil
}

// CopyPieceData 把信息拷贝到task的data里
func CopyPieceData(index int, buf []byte, msg *PeerMsg) (int, error) {
	if msg.Id != MsgPiece {
		return 0, fmt.Errorf("expected MsgPiece (Id %d), got Id %d", MsgPiece, msg.Id)
	}
	if len(msg.Payload) < 8 {
		return 0, fmt.Errorf("payload too short. %d < 8", len(msg.Payload))
	}
	// 解析序号，偏移，拷贝数据
	if parsedIndex := int(binary.BigEndian.Uint32(msg.Payload[0:4])); parsedIndex != index {
		return 0, fmt.Errorf("expected index %d, got %d", index, parsedIndex)
	}
	offset := int(binary.BigEndian.Uint32(msg.Payload[4:8]))
	if offset >= len(buf) {
		return 0, fmt.Errorf("offset too high. %d < %d", offset, len(buf))
	}
	data := msg.Payload[8:]
	if offset+len(data) > len(buf) {
		return 0, fmt.Errorf("data too large [%d] for offset %d with length %d", len(data), offset, len(buf))
	}
	copy(buf[offset:], data)
	return len(data), nil
}
