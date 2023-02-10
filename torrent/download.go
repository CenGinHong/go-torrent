package torrent

import (
	"bytes"
	"crypto/sha1"
	"log"
	"os"
	"time"
)

type TorrentTask struct {
	PeerId   [20]byte     // 客户端id
	PeerList []PeerInfo   // 从tracker获取到的一堆peer
	InfoSHA  [SHALEN]byte // 要下载文件的sha
	FileName string       // 文件名
	FileLen  int          // 文件长度
	PieceLen int
	PieceSHA [][SHALEN]byte
}

// 拆解后的每一个piece的task
type pieceTask struct {
	index  int // 序号
	sha    [SHALEN]byte
	length int // 长度，但如果是最后一篇可能会短一些
}

// 标准生产者消费者模型

// 表示下载的中间状态
type taskState struct {
	index      int       // 序号
	conn       *PeerConn // 跟peer建立的conn
	requested  int       // 发送多少请求
	downloaded int       // 接受多少字段
	backlog    int       // 并发度，client去peer一次请求一个block,是4k???
	data       []byte
}

func (s *taskState) handleMsg() error {
	msg, err := s.conn.ReadMsg()
	if err != nil {
		return err
	}
	if msg == nil {
		return nil
	}
	switch msg.Id {
	case MsgChoke:
		// peer不愿上传，该流程的piece放弃
		s.conn.Choked = true
	case MsgUnchoke:
		s.conn.Choked = false
	case MsgHave:
		index, err := GetHaveIndex(msg)
		if err != nil {
			return err
		}
		s.conn.Field.SetPiece(index)
	case MsgPiece:
		n, err := CopyPieceData(s.index, s.data, msg)
		if err != nil {
			return err
		}
		// 累加下载完成的
		s.downloaded += n
		s.backlog--
	}
	return nil
}

// getPieceBounds 切分下载的长度，指定开始和结束位置
func (t *TorrentTask) getPieceBounds(idx int) (begin, end int) {
	begin = idx * t.PieceLen
	end = begin + t.FileLen
	if end > t.FileLen {
		end = t.FileLen
	}
	return begin, end
}

// 放到channel，做sha的校验，校验成功就放到最后的data
type pieceResult struct {
	index int
	data  []byte
}

const (
	BlockSize  = 16384
	MaxBacklog = 5
)

func Download(task *TorrentTask) error {
	log.Printf("start downing %s\n", task.FileName)
	taskCh := make(chan *pieceTask, len(task.PieceSHA))
	defer close(taskCh)
	// 长度保持为1就好，即无缓存
	resultCh := make(chan *pieceResult)
	defer close(resultCh)
	for idx, sha := range task.PieceSHA {
		// 拆分成小的pieceTask
		begin, end := task.getPieceBounds(idx)
		// 虽然分片长度固定，但最后一个可能会比较短
		taskCh <- &pieceTask{idx, sha, end - begin}
	}
	for _, peer := range task.PeerList {
		go task.peerRoutine(peer, taskCh, resultCh)
	}
	// 存下载数据，按道理是应该存于io文件，但是toy项目就存内存吧
	buf := make([]byte, task.FileLen)
	count := 0
	for count < len(task.PieceSHA) {
		res := <-resultCh
		begin, end := task.getPieceBounds(res.index)
		copy(buf[begin:end], res.data)
		count++
		percent := float64(count) / float64(len(task.PieceSHA)) * 100
		log.Printf("downloading, progress: (%0.2f%%)\n", percent)
	}
	file, err := os.Create(task.FileName)
	if err != nil {
		log.Println("fail to create file: " + task.FileName)
		return err
	}
	_, err = file.Write(buf)
	if err != nil {
		log.Println("fail to write data")
		return err
	}
	return nil
}

func (t *TorrentTask) peerRoutine(peer PeerInfo, taskCh chan *pieceTask, resultCh chan *pieceResult) {
	// 建立连接
	conn, err := NewConn(peer, t.InfoSHA, t.PeerId)
	if err != nil {
		log.Printf("failed to connect peer: %s\n", peer.IP.String())
		return
	}
	defer func() {
		_ = conn.Close()
	}()
	log.Printf("complete handshake with peer: %s\n", peer.IP.String())
	// 写入信息
	if _, err = conn.WriteMsg(&PeerMsg{MsgInterested, make([]byte, 0)}); err != nil {
		log.Println("failed to write interest message")
		return
	}
	for task := range taskCh {
		// 连接的peer没有这一片，放回channel
		if !conn.Field.HasPiece(task.index) {
			taskCh <- task
			continue
		}
		log.Printf("get task, index: %v, peer: %v\n", task.index, peer.IP.String())
		res, err := downloadPiece(conn, task)
		if err != nil {
			taskCh <- task
			log.Printf("fail to download piece: %v\n", err)
		}
		if !checkPiece(task, res) {
			taskCh <- task
			continue
		}
		resultCh <- res
	}
}

func checkPiece(task *pieceTask, res *pieceResult) bool {
	sha := sha1.Sum(res.data)
	if !bytes.Equal(task.sha[:], sha[:]) {
		log.Printf("check integrity failed, index :%v\n", res.index)
		return false
	}
	return true
}

func downloadPiece(conn *PeerConn, task *pieceTask) (*pieceResult, error) {
	// piece下载中间状态
	state := &taskState{
		index: task.index,
		conn:  conn,
		data:  make([]byte, task.length),
	}
	// deadline 15s
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	defer func() {
		_ = conn.SetDeadline(time.Time{})
	}()
	// 还没下载完
	for state.downloaded < task.length {
		// 对面一定不能是choked状态，unhoked表示对面是愿意上传的
		if !conn.Choked {
			// 看是不是大于最大并发度，看请求的字节数是否已经超过文件的长度
			for state.backlog < MaxBacklog && state.requested < task.length {
				length := BlockSize
				// 最后一块可能长度不对齐不到blocksize,往前缩一些
				if task.length-state.requested < length {
					length = task.length - state.requested
				}
				// 从哪里开始下，下多少
				msg := NewRequestMsg(state.index, state.requested, length)
				if _, err := state.conn.WriteMsg(msg); err != nil {
					return nil, err
				}
				state.backlog++
				state.requested += length
			}
			if err := state.handleMsg(); err != nil {
				return nil, err
			}
		}
	}
	return &pieceResult{state.index, state.data}, nil
}
