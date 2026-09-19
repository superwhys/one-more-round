package photos

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"

	"github.com/superwhys/one-more-round/internal/errcode"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// encodePhoto strips upload metadata and produces the large image and thumbnail.
func encodePhoto(ctx context.Context, r io.Reader) ([2][]byte, error) {
	var output [2][]byte
	data, err := io.ReadAll(io.LimitReader(r, 10*1024*1024+1))
	if err != nil {
		return output, err
	}
	if len(data) > 10*1024*1024 {
		return output, errcode.ErrPhotoTooLarge
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 40000000 {
		return output, errcode.ErrPhotoUpload
	}
	if format != "jpeg" && format != "png" && format != "webp" {
		return output, errcode.ErrPhotoUpload
	}
	if err = ctx.Err(); err != nil {
		return output, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return output, errcode.ErrPhotoUpload
	}
	for i, size := range []int{2400, 480} {
		if err = ctx.Err(); err != nil {
			return output, err
		}
		w, h := cfg.Width, cfg.Height
		if w > size || h > size {
			if w >= h {
				h = max(1, h*size/w)
				w = size
			} else {
				w = max(1, w*size/h)
				h = size
			}
		}
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)
		var buf bytes.Buffer
		if err = jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85}); err != nil {
			return output, err
		}
		output[i] = buf.Bytes()
	}
	return output, nil
}
