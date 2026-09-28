package appicon

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func sizes(t *testing.T) map[int][]byte {
	t.Helper()
	images := map[int][]byte{}
	for i := 0; i < int(binary.LittleEndian.Uint16(ico[4:])); i++ {
		entry := ico[6+16*i:]
		w := int(entry[0])
		if w == 0 {
			w = 256
		}
		offset := binary.LittleEndian.Uint32(entry[12:])
		images[w] = ico[offset : offset+binary.LittleEndian.Uint32(entry[8:])]
	}
	if len(images) == 0 {
		t.Fatal("awlauncher.ico has no images")
	}
	return images
}

func TestImagePicksTheNearestLargerSize(t *testing.T) {
	images := sizes(t)
	largest := 0
	for w := range images {
		largest = max(largest, w)
	}
	for _, want := range []int{1, 16, 24, 32, 48, 64, 256, 1000} {
		got, err := Image(want)
		if err != nil {
			t.Fatal(err)
		}
		best := largest
		for w := range images {
			if w >= want && w < best {
				best = w
			}
		}
		if !bytes.Equal(got, images[best]) {
			t.Errorf("size %d: got another image than the %d px one", want, best)
		}
	}
}
