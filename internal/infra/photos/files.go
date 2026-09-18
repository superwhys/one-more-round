package photos

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

type Files struct{ Root string }

var identifier = regexp.MustCompile(`^[a-f0-9]{64}$`)

func (f *Files) Save(ctx context.Context, id string, r io.Reader) error {
	if !identifier.MatchString(id) {
		return errors.New("invalid photo ID")
	}
	data, e := io.ReadAll(io.LimitReader(r, 10*1024*1024+1))
	if e != nil {
		return e
	}
	if len(data) > 10*1024*1024 {
		return errors.New("照片不能超过 10 MB")
	}
	cfg, format, e := image.DecodeConfig(bytes.NewReader(data))
	if e != nil || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 40000000 {
		return errors.New("图片无效或超过 4000 万像素")
	}
	if format != "jpeg" && format != "png" && format != "webp" {
		return errors.New("仅支持 JPEG、PNG、WebP")
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	img, _, e := image.Decode(bytes.NewReader(data))
	if e != nil {
		return errors.New("图片解码失败")
	}
	if e = os.MkdirAll(f.Root, 0700); e != nil {
		return e
	}
	for _, size := range []int{2400, 480} {
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
		suffix := ".jpg"
		if size == 480 {
			suffix = "-thumb.jpg"
		}
		file, e := os.OpenFile(filepath.Join(f.Root, id+suffix), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			f.Remove(id)
			return e
		}
		e = jpeg.Encode(file, dst, &jpeg.Options{Quality: 85})
		closeErr := file.Close()
		if e != nil || closeErr != nil {
			f.Remove(id)
			return errors.New("图片保存失败")
		}
	}
	return nil
}
func (f *Files) Read(id string, thumb bool) ([]byte, error) {
	if !identifier.MatchString(id) {
		return nil, os.ErrNotExist
	}
	suffix := ".jpg"
	if thumb {
		suffix = "-thumb.jpg"
	}
	return os.ReadFile(filepath.Join(f.Root, id+suffix))
}
func (f *Files) Remove(id string) {
	if identifier.MatchString(id) {
		os.Remove(filepath.Join(f.Root, id+".jpg"))
		os.Remove(filepath.Join(f.Root, id+"-thumb.jpg"))
	}
}
