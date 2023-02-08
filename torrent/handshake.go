package torrent

import (
	"fmt"
	"io"
)

const (
	Reserved = 8
	HsMsgLen = Reserved + SHALEN + IDLEN
)

// HandshakeMsg ori:握手消息分为五块，1：指定第二段长度，2：什么协议，3：预留扩展，4：想要下载文件的hash，5：client id
type HandshakeMsg struct {
	PreStr  string
	InfoSHA [SHALEN]byte
	PeerId  [IDLEN]byte
}

// NewHandshakeMsg 新建hash握手信息
func NewHandshakeMsg(infoSHA, peerId [IDLEN]byte) *HandshakeMsg {
	return &HandshakeMsg{
		PreStr:  "BitTorrent protocol",
		InfoSHA: infoSHA,
		PeerId:  peerId,
	}
}

// WriteHandshake HandshakeMsg ori:握手消息分为五块，1：指定第二段长度，2：什么协议，3：预留扩展，4：想要下载文件的hash，5：client id
func WriteHandshake(w io.Writer, msg *HandshakeMsg) (int, error) {
	// 1 byte for prelen,共68个byte
	buf := make([]byte, len(msg.PreStr)+HsMsgLen+1)
	buf[0] = byte(len(msg.PreStr))
	curr := 1
	curr += copy(buf[curr:], msg.PreStr)
	curr += copy(buf[curr:], make([]byte, Reserved))
	curr += copy(buf[curr:], msg.InfoSHA[:])
	curr += copy(buf[curr:], msg.PeerId[:])
	return w.Write(buf)
}

func ReadHandshake(r io.Reader) (*HandshakeMsg, error) {
	lenBuf := make([]byte, 1)
	_, err := io.ReadFull(r, lenBuf)
	if err != nil {
		return nil, err
	}
	// prelen用于指示后面协议的位数
	prelen := int(lenBuf[0])
	if prelen == 0 {
		err = fmt.Errorf("prelen cannot be 0")
		return nil, err
	}

	// 用于填充剩余协议的四部分
	msgBuf := make([]byte, HsMsgLen+prelen)
	if _, err = io.ReadFull(r, msgBuf); err != nil {
		return nil, err
	}

	var peerId [IDLEN]byte
	var infoSHA [SHALEN]byte

	copy(infoSHA[:], msgBuf[prelen+Reserved:prelen+Reserved+SHALEN])
	copy(peerId[:], msgBuf[prelen+Reserved+SHALEN:])

	return &HandshakeMsg{
		PreStr:  string(msgBuf[0:prelen]),
		InfoSHA: infoSHA,
		PeerId:  peerId,
	}, nil
}
