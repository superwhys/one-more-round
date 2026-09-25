package config

import (
	"errors"
	"net/mail"
	"net/url"
	"strings"

	"github.com/miebyte/goutils/mysqlutils"

	smtp "github.com/superwhys/one-more-round/internal/infra/mail"
	"github.com/superwhys/one-more-round/internal/infra/photos"
	"github.com/superwhys/one-more-round/internal/infra/wechat"
)

type Runtime struct {
	Listen string                 `json:"-"`
	IsProd bool                   `json:"is_prod"`
	MySQL  mysqlutils.MysqlConfig `json:"mysql"`
	SMTP   smtp.Config            `json:"smtp"`
	Origin string                 `json:"origin"`
	OSS    photos.OSSConfig       `json:"oss"`
	Wechat wechat.Config          `json:"wechat"`
	BGG    BGGConfig              `json:"bgg"`
}

// BGGConfig holds the server-side BoardGameGeek application token. An empty
// token leaves external search unavailable and manual games usable.
type BGGConfig struct {
	Token string `json:"token"`
}

// Enabled reports whether an application token is configured.
func (c BGGConfig) Enabled() bool { return strings.TrimSpace(c.Token) != "" }

func (c *Runtime) Validate() error {
	if c.Listen == "" {
		return errors.New("listen is required")
	}

	if c.MySQL.Instance == "" || c.MySQL.Database == "" || c.MySQL.Username == "" {
		return errors.New("app.mysql requires instance, database and username")
	}
	if c.MySQL.PoolSize <= 0 {
		c.MySQL.PoolSize = 10
	}
	if c.MySQL.Charset == "" {
		c.MySQL.Charset = "utf8mb4"
	}
	u, e := url.Parse(c.Origin)
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
		u.Path != "" ||
		(u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("app.origin must be an http(s) origin without path")
	}
	if u.Scheme == "http" && u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" &&
		u.Hostname() != "::1" {
		return errors.New("production origin requires HTTPS")
	}
	if e = c.Wechat.Validate(); e != nil {
		return e
	}
	if e = c.OSS.Validate(); e != nil {
		return e
	}
	a, e := mail.ParseAddress(c.SMTP.From)
	if e != nil || a.Address != c.SMTP.From || strings.ContainsAny(c.SMTP.From, "\r\n") {
		return errors.New("app.smtp.from must be a valid email")
	}
	if c.SMTP.Host == "" || c.SMTP.Port < 1 || c.SMTP.Port > 65535 {
		return errors.New("app.smtp requires host and valid port")
	}
	return nil
}
