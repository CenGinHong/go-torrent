package torrent

type Bitfield []byte

func (b Bitfield) HasPiece(index int) bool {
	byteIndex := index / 8
	offset := index % 8
	if byteIndex < 0 || byteIndex >= len(b) {
		return false
	}
	// 选中那个比特
	// 右移，如果选中的位有1的话，这个1会被移到最低端，如00100000移到0000001，和1相遇如果不为0就是有这一片
	return b[byteIndex]>>uint(7-offset)&1 != 0
}

func (b Bitfield) SetPiece(index int) {
	byteIndex := index / 8
	offset := index % 8
	if byteIndex < 0 || byteIndex >= len(b) {
		return
	}
	b[byteIndex] |= 1 << uint(7-offset)
}
