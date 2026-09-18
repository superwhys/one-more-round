package photos

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

func TestReencodeAndBounds(t *testing.T) {
	f := &Files{Root: t.TempDir()}
	id := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	img := image.NewRGBA(image.Rect(0, 0, 900, 600))
	img.Set(0, 0, color.White)
	var buf bytes.Buffer
	png.Encode(&buf, img)
	if e := f.Save(context.Background(), id, &buf); e != nil {
		t.Fatal(e)
	}
	for _, thumb := range []bool{false, true} {
		data, e := f.Read(id, thumb)
		if e != nil {
			t.Fatal(e)
		}
		cfg, format, e := image.DecodeConfig(bytes.NewReader(data))
		if e != nil || format != "jpeg" {
			t.Fatal("not normalized JPEG")
		}
		if thumb && cfg.Width != 480 {
			t.Fatal("not a thumbnail")
		}
	}
	if e := f.Save(context.Background(), "../escape", bytes.NewReader(nil)); e == nil {
		t.Fatal("path accepted")
	}
	if e := f.Save(context.Background(), id, bytes.NewReader([]byte("fake.jpg"))); e == nil {
		t.Fatal("invalid content accepted")
	}
	f.Remove(id)
	if _, e := f.Read(id, false); !os.IsNotExist(e) {
		t.Fatal("not removed")
	}
}
