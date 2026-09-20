package photos

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/png"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"github.com/superwhys/one-more-round/internal/errcode"
)

const testPhotoID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func pngPhoto(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 900, 600))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func testOSS(t *testing.T, handler http.HandlerFunc) *OSS {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	c := oss.LoadDefaultConfig().WithEndpoint(server.URL).WithRegion("cn-shenzhen").
		WithUsePathStyle(true).WithRetryMaxAttempts(1).WithLogLevel(oss.LogOff).
		WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test-id", "test-secret"))
	return &OSS{client: oss.NewClient(c), bucket: "test-bucket", prefix: "image/"}
}

func TestOSSReencodeReadAndRemove(t *testing.T) {
	var mu sync.Mutex
	objects := map[string][]byte{}
	store := testOSS(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if !strings.HasPrefix(r.URL.Path, "/test-bucket/image/"+testPhotoID) || r.Header.Get("Authorization") == "" {
			t.Error("incorrect object prefix or unsigned request")
		}
		switch r.Method {
		case http.MethodPut:
			if r.Header.Get("x-oss-object-acl") != "private" || r.Header.Get("Content-Type") != "image/jpeg" || r.Header.Get("Cache-Control") != "private, no-store" {
				t.Error("missing private JPEG headers")
			}
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
			}
			objects[r.URL.Path] = data
		case http.MethodGet:
			if b, ok := objects[r.URL.Path]; ok {
				w.Write(b)
			} else {
				w.WriteHeader(404)
				io.WriteString(w, "<Error><Code>NoSuchKey</Code></Error>")
			}
		case http.MethodDelete:
			delete(objects, r.URL.Path)
			w.WriteHeader(204)
		}
	})
	ctx := context.Background()
	if err := store.Save(ctx, testPhotoID, bytes.NewReader(pngPhoto(t))); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	count := len(objects)
	mu.Unlock()
	if count != 1 {
		t.Fatalf("expected exactly one object, got %d", count)
	}
	for _, thumb := range []bool{false, true} {
		content, err := store.Read(ctx, testPhotoID, thumb)
		if err != nil {
			t.Fatal(err)
		}
		cfg, format, err := image.DecodeConfig(content.Body)
		_, drainErr := io.Copy(io.Discard, content.Body)
		closeErr := content.Body.Close()
		want := 900
		if err != nil || drainErr != nil || closeErr != nil || format != "jpeg" || cfg.Width != want {
			t.Fatalf("unexpected image: %v %s %v", cfg, format, err)
		}
	}
	if err := store.Remove(ctx, testPhotoID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Read(ctx, testPhotoID, false); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing object: %v", err)
	}
	if err := store.Remove(ctx, testPhotoID); err != nil {
		t.Fatal("deletion must be idempotent", err)
	}
}

func TestOSSFailureMappingAndDeletionAttempts(t *testing.T) {
	var deletes atomic.Int32
	store := testOSS(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deletes.Add(1)
		}
		w.WriteHeader(403)
		io.WriteString(w, "<Error><Code>AccessDenied</Code><Message>secret-response-marker</Message><RequestId>test-request</RequestId></Error>")
	})
	ctx := context.Background()
	if _, err := store.Read(ctx, testPhotoID, false); !errors.Is(err, errcode.ErrPhotoStorage) || errors.Is(err, fs.ErrNotExist) || strings.Contains(err.Error(), "secret-response-marker") {
		t.Fatalf("storage error was hidden or leaked: %v", err)
	}
	if err := store.Remove(ctx, testPhotoID); err == nil || deletes.Load() != 2 {
		t.Fatalf("both object deletions must be attempted: %v, %d", err, deletes.Load())
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := store.Read(canceled, testPhotoID, false); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}

func TestOSSPartialUploadCanBeRemoved(t *testing.T) {
	var mu sync.Mutex
	objects := map[string][]byte{}
	store := testOSS(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodPut {
			objects[r.URL.Path], _ = io.ReadAll(r.Body)
			// The object exists, but the client receives an unsuccessful reply.
			w.WriteHeader(503)
			io.WriteString(w, "<Error><Code>ServiceUnavailable</Code></Error>")
		} else if r.Method == http.MethodDelete {
			delete(objects, r.URL.Path)
			w.WriteHeader(204)
		}
	})
	ctx := context.Background()
	if err := store.Save(ctx, testPhotoID, bytes.NewReader(pngPhoto(t))); !errors.Is(err, errcode.ErrPhotoStorage) {
		t.Fatalf("partial upload reported success: %v", err)
	}
	mu.Lock()
	count := len(objects)
	mu.Unlock()
	if count != 1 {
		t.Fatal("fixture did not persist the first object")
	}
	if err := store.Remove(ctx, testPhotoID); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(objects) != 0 {
		t.Fatal("partial upload not cleaned")
	}
}

func TestOSSRejectsInvalidUploadsBeforeNetwork(t *testing.T) {
	store := testOSS(t, func(w http.ResponseWriter, r *http.Request) { t.Error("invalid upload reached OSS") })
	oversized := append([]byte(nil), pngPhoto(t)...)
	binary.BigEndian.PutUint32(oversized[16:20], 10000)
	binary.BigEndian.PutUint32(oversized[20:24], 10000)
	binary.BigEndian.PutUint32(oversized[29:33], crc32.ChecksumIEEE(oversized[12:29]))
	for name, data := range map[string][]byte{"fake": []byte("fake.jpg"), "too-large": make([]byte, 2*1024*1024+1), "too-many-pixels": oversized} {
		t.Run(name, func(t *testing.T) {
			if err := store.Save(context.Background(), testPhotoID, bytes.NewReader(data)); err == nil {
				t.Fatal("invalid photo accepted")
			}
		})
	}
	if err := store.Save(context.Background(), "../escape", bytes.NewReader(pngPhoto(t))); err == nil {
		t.Fatal("invalid ID accepted")
	}
}

func TestOSSConfigValidation(t *testing.T) {
	valid := OSSConfig{Bucket: "test-bucket", Region: "cn-shenzhen", Endpoint: "https://test-bucket.oss-cn-shenzhen.aliyuncs.com", Prefix: "image", AccessID: "test-id", AccessSecret: "test-secret"}
	if err := valid.Validate(); err != nil || valid.Endpoint != "https://oss-cn-shenzhen.aliyuncs.com" || valid.Prefix != "image/" {
		t.Fatalf("normalization failed: %v", err)
	}
	for name, change := range map[string]func(*OSSConfig){
		"missing-id":     func(c *OSSConfig) { c.AccessID = "" },
		"missing-secret": func(c *OSSConfig) { c.AccessSecret = "" },
		"http":           func(c *OSSConfig) { c.Endpoint = "http://oss-cn-shenzhen.aliyuncs.com" },
		"wrong-region":   func(c *OSSConfig) { c.Region = "cn-hangzhou" },
		"wildcard":       func(c *OSSConfig) { c.Prefix = "image/*" },
		"traversal":      func(c *OSSConfig) { c.Prefix = "../image/" },
		"bucket":         func(c *OSSConfig) { c.Bucket = "../bucket" },
	} {
		t.Run(name, func(t *testing.T) {
			c := valid
			change(&c)
			if err := c.Validate(); err == nil || strings.Contains(err.Error(), "test-secret") {
				t.Fatal("invalid or unsafe configuration result")
			}
		})
	}
}
