package formats

import (
	"encoding/binary"
	"fmt"
	"io"
)

func ReadMap(r io.Reader, width, height int) ([]uint32, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid map dimensions %dx%d", width, height)
	}
	count := width * height
	data := make([]byte, count*4)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, fmt.Errorf("read %dx%d map: %w", width, height, err)
	}
	var extra [1]byte
	if n, err := r.Read(extra[:]); n != 0 || err != io.EOF {
		return nil, fmt.Errorf("map contains data beyond %d bytes", len(data))
	}
	values := make([]uint32, count)
	for i := range values {
		values[i] = binary.LittleEndian.Uint32(data[i*4:])
	}
	return values, nil
}
