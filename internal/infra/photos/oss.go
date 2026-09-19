package photos

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/errcode"
)

// OSSConfig is read from app.oss in the private runtime configuration.
type OSSConfig struct {
	Bucket       string `json:"bucket"`
	Region       string `json:"region"`
	Endpoint     string `json:"endpoint"`
	Prefix       string `json:"prefix"`
	AccessID     string `json:"access_id"`
	AccessSecret string `json:"access_secret"`
}

func (c *OSSConfig) Validate() error {
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`).MatchString(c.Bucket) {
		return errors.New("app.oss.bucket is invalid")
	}
	if !regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)+$`).MatchString(c.Region) {
		return errors.New("app.oss.region is required")
	}
	u, err := url.Parse(c.Endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.Port() != "" {
		return errors.New("app.oss.endpoint must be an HTTPS endpoint without a path")
	}
	// Accept the bucket domain copied from the console, but give the SDK the
	// service endpoint so it does not prepend the bucket name twice.
	u.Host = strings.TrimPrefix(u.Host, c.Bucket+".")
	if u.Host != "oss-"+c.Region+".aliyuncs.com" && u.Host != "oss-"+c.Region+"-internal.aliyuncs.com" {
		return errors.New("app.oss.endpoint must match the configured region")
	}
	c.Endpoint = u.String()
	if c.Prefix == "" || strings.HasPrefix(c.Prefix, "/") || strings.ContainsAny(c.Prefix, "*?\\\r\n") {
		return errors.New("app.oss.prefix must be an object directory without wildcards")
	}
	for _, part := range strings.Split(strings.TrimSuffix(c.Prefix, "/"), "/") {
		if part == "" || part == "." || part == ".." {
			return errors.New("app.oss.prefix contains an invalid directory")
		}
	}
	c.Prefix = strings.TrimSuffix(c.Prefix, "/") + "/"
	if strings.TrimSpace(c.AccessID) == "" || strings.TrimSpace(c.AccessSecret) == "" {
		return errors.New("app.oss requires access_id and access_secret in private config.json")
	}
	return nil
}

// OSS stores only re-encoded JPEGs in a private bucket. Authorization stays in
// PhotoApp; clients never receive OSS addresses or credentials.
type OSS struct {
	client *oss.Client
	bucket string
	prefix string
}

func NewOSS(c OSSConfig) (*OSS, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	config := oss.LoadDefaultConfig().
		WithRegion(c.Region).
		WithEndpoint(c.Endpoint).
		WithCredentialsProvider(credentials.NewStaticCredentialsProvider(c.AccessID, c.AccessSecret)).
		WithConnectTimeout(5 * time.Second).
		WithReadWriteTimeout(10 * time.Second).
		WithRetryMaxAttempts(3).
		WithLogLevel(oss.LogOff)
	return &OSS{client: oss.NewClient(config), bucket: c.Bucket, prefix: c.Prefix}, nil
}

func (s *OSS) Save(ctx context.Context, id string, r io.Reader) error {
	if !identifier.MatchString(id) {
		return errcode.ErrPhotoUpload
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	images, err := encodePhoto(ctx, r)
	if err != nil {
		return err
	}
	for i, data := range images {
		_, err = s.client.PutObject(ctx, &oss.PutObjectRequest{
			Bucket: oss.Ptr(s.bucket), Key: oss.Ptr(s.prefix + photoName(id, i == 1)),
			Body: bytes.NewReader(data), ContentType: oss.Ptr("image/jpeg"),
			CacheControl: oss.Ptr("private, no-store"), Acl: oss.ObjectACLPrivate,
		})
		if err != nil {
			// The application tracks the ID before starting either upload, so
			// partially saved objects remain eligible for durable cleanup.
			return ossError(err)
		}
	}
	return nil
}

func (s *OSS) Read(ctx context.Context, id string, thumb bool) (*ports.PhotoContent, error) {
	if !identifier.MatchString(id) {
		return nil, fs.ErrNotExist
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	result, err := s.client.GetObject(ctx, &oss.GetObjectRequest{
		Bucket: oss.Ptr(s.bucket), Key: oss.Ptr(s.prefix + photoName(id, thumb)),
	})
	if err != nil {
		cancel()
		var serviceErr *oss.ServiceError
		if errors.As(err, &serviceErr) && serviceErr.Code == "NoSuchKey" {
			return nil, fs.ErrNotExist
		}
		return nil, ossError(err)
	}
	return &ports.PhotoContent{Body: &ossPhotoBody{ReadCloser: result.Body, cancel: cancel}, Size: result.ContentLength}, nil
}

// Keep the request deadline alive while the caller streams the response. Close
// cancels pending reads as well as releasing the underlying OSS connection.
type ossPhotoBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *ossPhotoBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		return n, ossError(err)
	}
	return n, err
}

func (b *ossPhotoBody) Close() error {
	b.cancel()
	if err := b.ReadCloser.Close(); err != nil {
		return ossError(err)
	}
	return nil
}

func (s *OSS) Remove(ctx context.Context, id string) error {
	if !identifier.MatchString(id) {
		return errcode.ErrPhotoNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var result error
	for _, thumb := range []bool{false, true} {
		_, err := s.client.DeleteObject(ctx, &oss.DeleteObjectRequest{
			Bucket: oss.Ptr(s.bucket), Key: oss.Ptr(s.prefix + photoName(id, thumb)),
		})
		if err != nil {
			result = errors.Join(result, ossError(err))
		}
	}
	return result
}

// Do not expose SDK response snapshots, signed request details or credentials.
func ossError(err error) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	var serviceErr *oss.ServiceError
	if errors.As(err, &serviceErr) {
		return fmt.Errorf("%w (OSS code %s, request %s)", errcode.ErrPhotoStorage, serviceErr.Code, serviceErr.RequestID)
	}
	return errcode.ErrPhotoStorage
}
