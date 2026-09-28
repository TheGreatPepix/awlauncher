package appicon

import (
	_ "embed"
	"encoding/binary"
	"errors"
)

//go:embed awlauncher.ico
var ico []byte

func ICO() []byte { return ico }

func Image(size int) ([]byte, error) {
	if len(ico) < 6 || binary.LittleEndian.Uint16(ico[2:]) != 1 {
		return nil, errors.New("awlauncher.ico is not an icon file")
	}
	count := int(binary.LittleEndian.Uint16(ico[4:]))
	best, bestSize := -1, 0
	better := func(w int) bool {
		switch {
		case best < 0:
			return true
		case (w >= size) != (bestSize >= size):
			return w >= size
		case w >= size:
			return w < bestSize
		}
		return w > bestSize
	}
	for i := 0; i < count; i++ {
		entry := 6 + 16*i
		if entry+16 > len(ico) {
			return nil, errors.New("awlauncher.ico is truncated")
		}
		w := int(ico[entry])
		if w == 0 {
			w = 256
		}
		if better(w) {
			best, bestSize = i, w
		}
	}
	if best < 0 {
		return nil, errors.New("awlauncher.ico has no images")
	}
	entry := ico[6+16*best:]
	length := int(binary.LittleEndian.Uint32(entry[8:]))
	offset := int(binary.LittleEndian.Uint32(entry[12:]))
	if offset < 0 || length <= 0 || offset+length > len(ico) {
		return nil, errors.New("awlauncher.ico is truncated")
	}
	return ico[offset : offset+length], nil
}
