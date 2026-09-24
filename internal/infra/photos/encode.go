package photos

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"

	_ "golang.org/x/image/webp"

	"github.com/superwhys/one-more-round/internal/domain/photo"
	"github.com/superwhys/one-more-round/internal/errcode"
)

// encodePhoto normalizes a client-sized image to one JPEG without metadata.
// Reject dimensions before decoding: compressed file size is not a memory bound.
func encodePhoto(ctx context.Context, r io.Reader) ([]byte, error) {
	var output []byte
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(r, photo.MaxUploadBytes+1))
	if err != nil {
		return output, err
	}
	if len(data) > photo.MaxUploadBytes {
		return output, errcode.ErrPhotoTooLarge
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return output, errcode.ErrPhotoUpload
	}
	if cfg.Width > photo.MaxDimension || cfg.Height > photo.MaxDimension {
		return nil, errcode.ErrPhotoDimensions
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
	if img.Bounds().Dx() != cfg.Width || img.Bounds().Dy() != cfg.Height {
		return nil, errcode.ErrPhotoUpload
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 82}); err != nil {
		return nil, err
	}
	if buf.Len() > photo.MaxUploadBytes {
		return nil, errcode.ErrPhotoTooLarge
	}
	return buf.Bytes(), nil
}
