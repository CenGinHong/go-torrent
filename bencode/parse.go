package bencode

import (
	"bufio"
	"io"
)

func Parse(r io.Reader) (*BObject, error) {
	br := bufio.NewReader(r)
	// 查看第一个字符
	b, err := br.Peek(1)
	if err != nil {
		return nil, err
	}
	var ret BObject
	switch {
	case b[0] >= '0' && b[0] <= '9':
		val, err := DecodeString(br)
		if err != nil {
			return nil, err
		}
		ret.type_ = BSTR
		ret.val_ = val
	case b[0] == 'i':
		val, err := DecodeInt(br)
		if err != nil {
			return nil, err
		}
		ret.type_ = BINT
		ret.val_ = val
	case b[0] == 'l':
		// 读取掉l
		if _, err = br.ReadByte(); err != nil {
			return nil, err
		}
		list := make([]*BObject, 0)
		for {
			if p, _ := br.Peek(1); p[0] == 'e' {
				if _, err = br.ReadByte(); err != nil {
					return nil, err
				}
				break
			}
			// 递归下降
			elem, err := Parse(br)
			if err != nil {
				return nil, err
			}
			list = append(list, elem)
		}
		ret.type_ = BLIST
		ret.val_ = list
	case b[0] == 'd':
		if _, err = br.ReadByte(); err != nil {
			return nil, err
		}
		dict := make(map[string]*BObject)
		for {
			if p, _ := br.Peek(1); p[0] == 'e' {
				if _, err = br.ReadByte(); err != nil {
					return nil, err
				}
				break
			}
			key, err := DecodeString(br)
			if err != nil {
				return nil, err
			}
			val, err := Parse(br)
			if err != nil {
				return nil, err
			}
			dict[key] = val
		}
		ret.type_ = BDIST
		ret.val_ = dict
	default:
		return nil, ErrIvd
	}
	return &ret, nil
}
