package pngenc

// crc32 is the CRC-32 every PNG chunk carries. It is here rather than from
// hash/crc32 for the same reason the compressor is: four chunks' worth of
// checksum is not worth an import that drags a hardware-accelerated table
// builder into a binary that only ever draws one small picture.

var crcTable [256]uint32

func init() {
	for i := range crcTable {
		c := uint32(i)
		for k := 0; k < 8; k++ {
			if c&1 != 0 {
				c = 0xedb88320 ^ (c >> 1)
			} else {
				c >>= 1
			}
		}
		crcTable[i] = c
	}
}

func crc32(data []byte) uint32 {
	c := ^uint32(0)
	for _, b := range data {
		c = crcTable[byte(c)^b] ^ (c >> 8)
	}
	return ^c
}
