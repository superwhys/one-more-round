package photos

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

// Files is the local adapter used by isolated application tests.
type Files struct{ Root string }

var identifier = regexp.MustCompile(`^[a-f0-9]{64}$`)

func photoName(id string, thumb bool) string {
	if thumb {
		return id + "-thumb.jpg"
	}
	return id + ".jpg"
}

func (f *Files) Save(ctx context.Context, id string, r io.Reader) error {
	if !identifier.MatchString(id) {
		return errors.New("invalid photo ID")
	}
	images, err := encodePhoto(ctx, r)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(f.Root, 0700); err != nil {
		return err
	}
	for i, data := range images {
		if err = ctx.Err(); err != nil {
			return err
		}
		file, err := os.OpenFile(filepath.Join(f.Root, photoName(id, i == 1)), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, writeErr := file.Write(data)
		if err = errors.Join(writeErr, file.Close()); err != nil {
			return err
		}
	}
	return nil
}

func (f *Files) Read(ctx context.Context, id string, thumb bool) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !identifier.MatchString(id) {
		return nil, os.ErrNotExist
	}
	return os.ReadFile(filepath.Join(f.Root, photoName(id, thumb)))
}

func (f *Files) Remove(ctx context.Context, id string) error {
	if !identifier.MatchString(id) {
		return errors.New("invalid photo ID")
	}
	var result error
	for _, thumb := range []bool{false, true} {
		if err := ctx.Err(); err != nil {
			return errors.Join(result, err)
		}
		if err := os.Remove(filepath.Join(f.Root, photoName(id, thumb))); err != nil && !os.IsNotExist(err) {
			result = errors.Join(result, err)
		}
	}
	return result
}
