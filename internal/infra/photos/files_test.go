package photos

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"testing"

	"github.com/superwhys/one-more-round/internal/errcode"
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
	entries, err := os.ReadDir(f.Root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected a single stored image: %v %v", entries, err)
	}
	for _, thumb := range []bool{false, true} {
		content, e := f.Read(context.Background(), id, thumb)
		if e != nil {
			t.Fatal(e)
		}
		cfg, format, e := image.DecodeConfig(content.Body)
		_, drainErr := io.Copy(io.Discard, content.Body)
		closeErr := content.Body.Close()
		if e != nil || drainErr != nil || closeErr != nil || format != "jpeg" {
			t.Fatal("not normalized JPEG")
		}
		if cfg.Width != 900 {
			t.Fatal("both URLs must return the display image")
		}
	}
	if e := f.Save(context.Background(), "../escape", bytes.NewReader(nil)); e == nil {
		t.Fatal("path accepted")
	}
	if e := f.Save(context.Background(), id, bytes.NewReader([]byte("fake.jpg"))); e == nil {
		t.Fatal("invalid content accepted")
	}
	f.Remove(context.Background(), id)
	if _, e := f.Read(context.Background(), id, false); !os.IsNotExist(e) {
		t.Fatal("not removed")
	}
}

func TestUploadSizeBoundary(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{2*1024*1024 - 1, 2 * 1024 * 1024, 2*1024*1024 + 1} {
		data := make([]byte, size)
		copy(data, encoded.Bytes())
		_, err := encodePhoto(context.Background(), bytes.NewReader(data))
		if size <= 2*1024*1024 && err != nil {
			t.Fatalf("valid %d-byte photo rejected: %v", size, err)
		}
		if size > 2*1024*1024 && !errors.Is(err, errcode.ErrPhotoTooLarge) {
			t.Fatalf("oversized %d-byte photo must be rejected, got %v", size, err)
		}
	}
}
