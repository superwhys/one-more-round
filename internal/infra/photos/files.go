package photos

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/superwhys/one-more-round/internal/app/ports"
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
	data, err := encodePhoto(ctx, r)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(f.Root, 0o700); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	file, err := os.OpenFile(
		filepath.Join(f.Root, photoName(id, false)),
		os.O_CREATE|os.O_EXCL|os.O_WRONLY,
		0o600,
	)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	return errors.Join(writeErr, file.Close())
}

func (f *Files) Read(ctx context.Context, id string, _ bool) (*ports.PhotoContent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !identifier.MatchString(id) {
		return nil, os.ErrNotExist
	}
	file, err := os.Open(filepath.Join(f.Root, photoName(id, false)))
	if os.IsNotExist(err) {
		file, err = os.Open(filepath.Join(f.Root, photoName(id, true)))
	}
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	return &ports.PhotoContent{Body: file, Size: info.Size()}, nil
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
		if err := os.Remove(
			filepath.Join(f.Root, photoName(id, thumb)),
		); err != nil &&
			!os.IsNotExist(err) {
			result = errors.Join(result, err)
		}
	}
	return result
}
