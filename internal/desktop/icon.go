package desktop

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"runtime"
)

// Original simple S mark; generated locally, no external image asset.
func icon() []byte {
	im := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	green := color.NRGBA{76, 122, 30, 255}
	for y := 5; y < 28; y++ {
		for x := 6; x < 26; x++ {
			if y < 9 || y >= 24 || (y >= 14 && y < 18) || (x < 10 && y < 18) || (x >= 22 && y >= 14) {
				im.Set(x, y, green)
			}
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, im)
	data := b.Bytes()
	if runtime.GOOS != "windows" {
		return data
	}
	var ico bytes.Buffer
	_ = binary.Write(&ico, binary.LittleEndian, []uint16{0, 1, 1})
	ico.Write([]byte{32, 32, 0, 0})
	_ = binary.Write(&ico, binary.LittleEndian, []uint16{1, 32})
	_ = binary.Write(&ico, binary.LittleEndian, []uint32{uint32(len(data)), 22})
	ico.Write(data)
	return ico.Bytes()
}
