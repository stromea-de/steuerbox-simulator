package report

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"image"
	"image/color"
)

// Flatten transparent PNG pixels onto white and embed compact RGB via Flate.
func imageObject(im image.Image) string {
	bounds := im.Bounds()
	var raw bytes.Buffer
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.NRGBAModel.Convert(im.At(x, y)).(color.NRGBA)
			a := uint32(c.A)
			for _, v := range []uint8{c.R, c.G, c.B} {
				raw.WriteByte(byte((uint32(v)*a + 255*(255-a) + 127) / 255))
			}
		}
	}
	var compressed bytes.Buffer
	w := zlib.NewWriter(&compressed)
	_, _ = w.Write(raw.Bytes())
	_ = w.Close()
	return fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /FlateDecode /Length %d >>\nstream\n%s\nendstream", bounds.Dx(), bounds.Dy(), compressed.Len(), compressed.Bytes())
}
