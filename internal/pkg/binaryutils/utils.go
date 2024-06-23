package binaryutils

import (
	"encoding/binary"
	"errors"
	"io"
)

// DeserializeVLQ reads bytes from a *bytes.Buffer and interprets them as a variable-length quantity (VLQ).
// It returns the decoded uint64 number and the number of bytes read.
func DeserializeVLQ(buf io.ByteReader) (uint64, int, error) {
	var n uint64
	var size int

	for {
		val, err := buf.ReadByte()
		if err != nil {
			if err == io.EOF {
				break // End of stream is a natural break
			}
			return 0, 0, err // Return any error that occurs
		}

		size++
		n = (n << 7) | uint64(val&0x7F)
		if val&0x80 != 0x80 {
			break
		}
		n++
	}

	return n, size, nil
}

func ReverseBytesWithCopy(input []byte) []byte {
	temp := make([]byte, len(input))
	copy(temp, input)
	for i, j := 0, len(temp)-1; i < j; i, j = i+1, j-1 {
		temp[i], temp[j] = temp[j], temp[i]
	}
	return temp
}

func DecodeVarint(data []byte) (int, uint, error) {
	switch size := data[0]; {
	case size < 253:
		return 1, uint(size), nil
	case size == 253:
		num := binary.LittleEndian.Uint16(data[1:3])
		return 3, uint(num), nil
	case size == 254:
		num := binary.LittleEndian.Uint32(data[1:5])
		return 5, uint(num), nil
	case size == 255:
		num := binary.LittleEndian.Uint64(data[1:9])
		return 9, uint(num), nil
	default:
		return 0, 0, errors.New("can't decode this")
	}
}
