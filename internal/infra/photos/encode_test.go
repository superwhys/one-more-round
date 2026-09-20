package photos

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/superwhys/one-more-round/internal/errcode"
)

func TestDisplayPhotoDimensionsAndMetadata(t *testing.T) {
	var input bytes.Buffer
	if err := png.Encode(&input, image.NewNRGBA64(image.Rect(0, 0, 1600, 1600))); err != nil {
		t.Fatal(err)
	}
	input.WriteString("private-metadata-marker")
	output, err := encodePhoto(context.Background(), bytes.NewReader(input.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(output))
	if err != nil || format != "jpeg" || cfg.Width != 1600 || cfg.Height != 1600 || bytes.Contains(output, []byte("private-metadata-marker")) {
		t.Fatalf("unexpected display image: %v %s %v", cfg, format, err)
	}
	for _, dimensions := range [][2]uint32{{1601, 1}, {1, 1601}, {5000, 5000}} {
		// Valid header, intentionally missing pixel data: reject before decoding.
		data := bytes.Clone(input.Bytes()[:33])
		binary.BigEndian.PutUint32(data[16:20], dimensions[0])
		binary.BigEndian.PutUint32(data[20:24], dimensions[1])
		binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))
		if _, err := encodePhoto(context.Background(), bytes.NewReader(data)); !errors.Is(err, errcode.ErrPhotoDimensions) {
			t.Fatalf("%v must be rejected before pixel decoding: %v", dimensions, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := encodePhoto(ctx, bytes.NewReader(input.Bytes())); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled processing: %v", err)
	}
}

// Opt in without the race detector: measure repeated accepted worst-bit-depth
// inputs independently of other tests and of the fixture's allocation.
func TestDisplayPhotoMemoryBudget(t *testing.T) {
	if os.Getenv("OMR_PHOTO_MEMORY_TEST") != "1" {
		t.Skip("set OMR_PHOTO_MEMORY_TEST=1 and run only this test without -race")
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA64(image.Rect(0, 0, 1600, 1600))); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 2*1024*1024)
	copy(data, encoded.Bytes())
	encoded = bytes.Buffer{}
	runtime.GC()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)
	var peak atomic.Uint64
	peak.Store(baseline.HeapAlloc)
	stop, stopped := make(chan struct{}), make(chan struct{})
	sample := func() {
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		for old := peak.Load(); stats.HeapAlloc > old && !peak.CompareAndSwap(old, stats.HeapAlloc); old = peak.Load() {
		}
	}
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				sample()
			case <-stop:
				return
			}
		}
	}()
	defer func() { close(stop); <-stopped }()
	for range 3 {
		if _, err := encodePhoto(context.Background(), bytes.NewReader(data)); err != nil {
			t.Fatal(err)
		}
		sample()
	}
	delta := peak.Load() - baseline.HeapAlloc
	t.Logf("three serial 1600x1600 16-bit PNGs, 2 MiB each: peak additional Go heap %.1f MiB", float64(delta)/(1024*1024))
	if delta > 100*1024*1024 {
		t.Fatalf("photo processing exceeds 100 MiB heap budget: %d", delta)
	}
}

// Run without -race to measure processing allocations, excluding fixture setup.
// Total allocated bytes provide a conservative bound on this operation's live heap.
func BenchmarkDisplayPhotoMemory(b *testing.B) {
	for _, format := range []string{"png8", "png16", "jpeg"} {
		b.Run(format, func(b *testing.B) {
			var input bytes.Buffer
			var err error
			switch format {
			case "png16":
				err = png.Encode(&input, image.NewNRGBA64(image.Rect(0, 0, 1600, 1600)))
			case "png8":
				err = png.Encode(&input, image.NewNRGBA(image.Rect(0, 0, 1600, 1600)))
			default:
				err = jpeg.Encode(&input, image.NewRGBA(image.Rect(0, 0, 1600, 1600)), nil)
			}
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := encodePhoto(context.Background(), bytes.NewReader(input.Bytes())); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
