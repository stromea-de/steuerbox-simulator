package inspection

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func TestLocalProfileImageBounds024b(t *testing.T) {
	for _, size := range []int{1, 640, 641} {
		im := image.NewNRGBA(image.Rect(0, 0, size, 2))
		im.Set(0, 0, color.NRGBA{G: 100, A: 255})
		var b bytes.Buffer
		if e := png.Encode(&b, im); e != nil {
			t.Fatal(e)
		}
		p := InspectorProfile{Logo: "data:image/png;base64," + base64.StdEncoding.EncodeToString(b.Bytes())}
		e := p.Validate()
		if (e == nil) != (size <= 640) {
			t.Fatal(size, e)
		}
	}
	for _, logo := range []string{"https://example.org/logo.png", "data:image/svg+xml;base64,PHN2Zz4=", "data:image/png;base64," + strings.Repeat("A", 1000001)} {
		if (InspectorProfile{Logo: logo}).Validate() == nil {
			t.Fatal("accepted invalid logo")
		}
	}
	if (InspectorProfile{Street: strings.Repeat("ü", 201)}).Validate() == nil {
		t.Fatal("unbounded own address")
	}
}

func TestEmptyProfileDoesNotChangeHistoricalJSON024b(t *testing.T) {
	b, e := json.Marshal(Config{})
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(b, []byte(`"profile":`)) {
		t.Fatal("new empty field changes historical report identity")
	}
	b, e = json.Marshal(Config{Profile: InspectorProfile{Business: "Elektro"}})
	if e != nil || !bytes.Contains(b, []byte(`"profile":{"business":"Elektro"}`)) {
		t.Fatal(string(b), e)
	}
}
