package bencode

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"
)

// BType 表示Bencode的四种数据类型
type BType uint8

var (
	ErrNum = errors.New("expect num")
	ErrCol = errors.New("expect colon")
	ErrEpI = errors.New("expect char i")
	ErrEpE = errors.New("expect char e")
	ErrTyp = errors.New("wrong type")
	ErrIvd = errors.New("invalid bencode")
)

const (
	BSTR  BType = 0x01
	BINT  BType = 0x02
	BLIST BType = 0x03
	BDICT BType = 0x04
)

type BValue interface{}

type BObject struct {
	type_ BType
	val_  BValue // 可能是字符串，int, slice指针，k为string,v是bvalue的map
}

func (o *BObject) Str() (string, error) {
	if o.type_ != BSTR {
		return "", ErrTyp
	}
	return o.val_.(string), nil
}

func (o *BObject) Int() (int, error) {
	if o.type_ != BINT {
		return 0, ErrTyp
	}
	return o.val_.(int), nil
}

func (o *BObject) List() ([]*BObject, error) {
	if o.type_ != BLIST {
		return nil, ErrTyp
	}
	return o.val_.([]*BObject), nil
}

func (o *BObject) Dict() (map[string]*BObject, error) {
	if o.type_ != BDICT {
		return nil, ErrTyp
	}
	return o.val_.(map[string]*BObject), nil
}

func (o *BObject) Bencode(w io.Writer) int {
	bw := bufio.NewWriter(w)
	wLen := 0
	switch o.type_ {
	case BSTR:
		// 先把内内容转字符串
		str, _ := o.Str()
		// 再把字符串转bencode
		wLen += EncodeString(bw, str)
	case BINT:
		val, _ := o.Int()
		wLen += EncodeInt(bw, val)
	case BLIST:
		_ = bw.WriteByte('l')
		// TODO 忽略错误？
		list, _ := o.List()
		// 递归下降
		for _, elem := range list {
			wLen += elem.Bencode(bw)
		}
		_ = bw.WriteByte('e')
		wLen += 2
	case BDICT:
		_ = bw.WriteByte('d')
		dict, _ := o.Dict()
		for k, v := range dict {
			wLen += EncodeString(bw, k)
			wLen += v.Bencode(bw)
		}
		if err := bw.WriteByte('e'); err != nil {
			return 0
		}
		wLen += 2
	}
	if err := bw.Flush(); err != nil {
		return 0
	}
	return wLen
}

func writeDecimal(w *bufio.Writer, val int) (len int) {
	// TODO 为什么不用itoa
	s := strconv.Itoa(val)
	valLen, err := w.WriteString(s)
	if err != nil {
		return 0
	}
	return valLen
	//if val == 0 {
	//	if err := w.WriteByte('0'); err != nil {
	//		return 0
	//	}
	//	len++
	//	return
	//}
	//if val < 0 {
	//	if err := w.WriteByte('-'); err != nil {
	//		return 0
	//	}
	//	len++
	//	val *= -1
	//}
	//// 这里是正序拆分
	//dividend := 1
	//for {
	//	if dividend > val {
	//		dividend /= 10
	//		break
	//	}
	//	dividend *= 10
	//}
	//for {
	//	num := byte(val / dividend)
	//	if err := w.WriteByte('0' + num); err != nil {
	//		return 0
	//	}
	//	len++
	//	if dividend == 1 {
	//		return
	//	}
	//	val %= dividend
	//	dividend /= 10
	//}
}

func readDecimal(r *bufio.Reader) (val int, len int) {
	// 正负数标志
	sb := strings.Builder{}
	b, err := r.ReadByte()
	if err != nil {
		return 0, 0
	}
	if b == '-' {
		sb.WriteByte(b)
		b, _ = r.ReadByte()
		if err != nil {
			return 0, 0
		}
	}
	for checkNum(b) {
		sb.WriteByte(b)
		if b, err = r.ReadByte(); err != nil {
			return 0, 0
		}
	}
	if err = r.UnreadByte(); err != nil {
		return 0, 0
	}
	if val, err = strconv.Atoi(sb.String()); err != nil {
		return 0, 0
	}
	return val, sb.Len()
	//sign := 1
	//// 读取一个byte
	//b, err := r.ReadByte()
	//if err != nil {
	//	return 0, 0
	//}
	//len++
	//// 如果是负数将标志置-1
	//if b == '-' {
	//	sign = -1
	//	b, _ = r.ReadByte()
	//	if err != nil {
	//		return 0, 0
	//	}
	//	len++
	//}
	//for {
	//	if !checkNum(b) {
	//		if err = r.UnreadByte(); err != nil {
	//			return 0, 0
	//		}
	//		len--
	//		return sign * val, len
	//	}
	//	val = val*10 + int(b-'0')
	//	b, _ = r.ReadByte()
	//	len++
	//}
}

func checkNum(data byte) bool {
	return data >= '0' && data <= '9'
}

func EncodeString(w io.Writer, val string) int {
	// 求出string的长度
	strLen := len(val)
	bw := bufio.NewWriter(w)
	defer func(bw *bufio.Writer) {
		_ = bw.Flush()
	}(bw)
	// 把长度转string后写入
	// TODO 返回了啥？
	// wlen是整个的长度，即3:abc的长度
	wLen := writeDecimal(bw, strLen)
	if err := bw.WriteByte(':'); err != nil {
		return 0
	}
	wLen++
	if _, err := bw.WriteString(val); err != nil {
		return 0
	}
	wLen += strLen
	return wLen
}

func DecodeString(r io.Reader) (val string, err error) {
	//br, ok := bufio.NewReader(r)
	//if !ok {
	//	br = bufio.NewReader(r)
	//}
	br := bufio.NewReader(r)
	// 将冒号之前的表示的数字读出来
	num, intLen := readDecimal(br)
	if intLen == 0 {
		return val, ErrNum
	}
	// 读冒号
	b, err := br.ReadByte()
	if err != nil {
		return "", err
	}
	if b != ':' {
		return val, ErrCol
	}
	// 读取接下来的字符串
	buf := make([]byte, num)
	if _, err = io.ReadAtLeast(br, buf, num); err != nil {
		return "", err
	}
	val = string(buf)
	return
}

func EncodeInt(w io.Writer, val int) int {
	bw := bufio.NewWriter(w)
	defer func(bw *bufio.Writer) {
		_ = bw.Flush()
	}(bw)
	wLen := 0
	if err := bw.WriteByte('i'); err != nil {
		return 0
	}
	wLen++
	nLen := writeDecimal(bw, val)
	wLen += nLen
	if err := bw.WriteByte('e'); err != nil {
		return 0
	}
	wLen++
	return wLen
}

func DecodeInt(r io.Reader) (val int, err error) {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}
	b, err := br.ReadByte()
	if b != 'i' {
		return val, ErrEpI
	}
	val, _ = readDecimal(br)
	if b, err = br.ReadByte(); err != nil {
		return 0, err
	}
	if b != 'e' {
		return val, ErrEpE
	}
	return
}
