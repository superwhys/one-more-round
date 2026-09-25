// Package wechat adapts the mini-program code2Session exchange to application identity.
package wechat

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/errcode"
)

const code2SessionURL = "https://api.weixin.qq.com/sns/jscode2session"

// Config contains only server-side WeChat application credentials.
type Config struct {
	AppID  string `json:"app_id"`
	Secret string `json:"secret"`
}

// Enabled reports whether the complete optional integration is configured.
func (c Config) Enabled() bool { return c.AppID != "" && c.Secret != "" }

// Validate rejects partial configuration instead of silently disabling login.
func (c Config) Validate() error {
	if c.AppID == "" && c.Secret == "" {
		return nil
	}
	if strings.TrimSpace(c.AppID) != c.AppID || strings.TrimSpace(c.Secret) != c.Secret ||
		c.AppID == "" || c.Secret == "" || len(c.AppID) > 64 {
		return errors.New("app.wechat requires app_id and secret together")
	}
	return nil
}

// Client exchanges codes with a bounded, non-redirecting server-side request.
type Client struct {
	config   Config
	http     *http.Client
	endpoint string
}

// New creates the WeChat adapter after configuration validation at startup.
func New(config Config) *Client {
	return &Client{config: config, endpoint: code2SessionURL, http: &http.Client{
		Timeout:       8 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

// ExchangeCode verifies a temporary code; raw provider errors are deliberately
// excluded because net/http errors can contain the secret-bearing request URL.
func (c *Client) ExchangeCode(ctx context.Context, code string) (ports.WechatIdentity, error) {
	if strings.TrimSpace(code) == "" || len(code) > 512 {
		return ports.WechatIdentity{}, errcode.ErrWechatCode
	}
	query := url.Values{"appid": {c.config.AppID}, "secret": {c.config.Secret},
		"js_code": {code}, "grant_type": {"authorization_code"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint+"?"+query.Encode(), nil)
	if err != nil {
		return ports.WechatIdentity{}, errcode.ErrWechatLogin
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return ports.WechatIdentity{}, errcode.ErrWechatLogin
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ports.WechatIdentity{}, errcode.ErrWechatLogin
	}
	var result struct {
		OpenID  string `json:"openid"`
		ErrCode int    `json:"errcode"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 8192)).Decode(&result); err != nil {
		return ports.WechatIdentity{}, errcode.ErrWechatLogin
	}
	if result.ErrCode == 40029 || result.ErrCode == 40163 || result.ErrCode == 40226 {
		return ports.WechatIdentity{}, errcode.ErrWechatCode
	}
	if result.ErrCode != 0 || strings.TrimSpace(result.OpenID) == "" || len(result.OpenID) > 128 {
		return ports.WechatIdentity{}, errcode.ErrWechatLogin
	}
	return ports.WechatIdentity{AppID: c.config.AppID, OpenID: result.OpenID}, nil
}
