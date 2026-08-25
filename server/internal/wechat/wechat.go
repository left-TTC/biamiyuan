// wechat.go
// 微信开放能力：code2Session（换取 openid）、access_token、订阅消息推送
//
// 说明：
//   - 需要【企业主体】小程序，并在微信公众平台申请「订阅消息」模板
//   - 通过环境变量配置（见 Config）
//   - 未配置凭据时，SendSubscribeMessage 返回 ErrNotConfigured，
//     上层将回退为「模拟推送」（仅记录日志）
package wechat

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ErrNotConfigured 表示未配置微信凭据
var ErrNotConfigured = errors.New("wechat: 未配置 appid/secret/template_id")

// Config 微信开放能力配置（来自环境变量）
type Config struct {
	AppID      string // WECHAT_APPID
	AppSecret  string // WECHAT_SECRET
	TemplateID string // WECHAT_TEMPLATE_ID（订阅消息模板 ID）
}

// Configured 是否已配置完整凭据
func (c Config) Configured() bool {
	return c.AppID != "" && c.AppSecret != "" && c.TemplateID != ""
}

// Client 微信 API 客户端
type Client struct {
	cfg         Config
	httpClient  *http.Client
	mu          sync.Mutex
	accessToken string
	tokenExpire time.Time
}

// Configured 是否已配置完整凭据
func (c *Client) Configured() bool {
	return c.cfg.Configured()
}

// TemplateID 返回订阅消息模板 ID
func (c *Client) TemplateID() string {
	return c.cfg.TemplateID
}

// New 创建微信客户端
func New(cfg Config) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// ---------- access_token ----------

type tokenResp struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

// AccessToken 获取 access_token（带缓存，提前 5 分钟过期）
func (c *Client) AccessToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.accessToken != "" && time.Now().Before(c.tokenExpire) {
		return c.accessToken, nil
	}

	url := fmt.Sprintf(
		"https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s",
		c.cfg.AppID, c.cfg.AppSecret,
	)
	var resp tokenResp
	if err := c.getJSON(url, &resp); err != nil {
		return "", err
	}
	if resp.ErrCode != 0 {
		return "", fmt.Errorf("wechat: access_token 失败 errcode=%d errmsg=%s", resp.ErrCode, resp.ErrMsg)
	}
	c.accessToken = resp.AccessToken
	c.tokenExpire = time.Now().Add(time.Duration(resp.ExpiresIn-300) * time.Second)
	return c.accessToken, nil
}

// ---------- code2Session ----------

type code2SessionResp struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

// Code2Session 用小程序 wx.login 的 code 换取 openid
func (c *Client) Code2Session(code string) (string, error) {
	if !c.cfg.Configured() {
		return "", ErrNotConfigured
	}
	url := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		c.cfg.AppID, c.cfg.AppSecret, code,
	)
	var resp code2SessionResp
	if err := c.getJSON(url, &resp); err != nil {
		return "", err
	}
	if resp.ErrCode != 0 || resp.OpenID == "" {
		return "", fmt.Errorf("wechat: code2Session 失败 errcode=%d errmsg=%s", resp.ErrCode, resp.ErrMsg)
	}
	return resp.OpenID, nil
}

// ---------- 订阅消息 ----------

type subscribeMsg struct {
	Touser     string                 `json:"touser"`
	TemplateID string                 `json:"template_id"`
	Page       string                 `json:"page"`
	Data       map[string]interface{} `json:"data"`
}

type sendResp struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// SendSubscribeMessage 发送订阅消息
//
// data 形如 {"thing1":{"value":"xxx"},"time2":{"value":"xx时xx分"}}
// 字段 key（thing1/time2 等）需与申请模板时的字段对应
func (c *Client) SendSubscribeMessage(openid, page string, data map[string]interface{}) error {
	if !c.cfg.Configured() {
		return ErrNotConfigured
	}
	token, err := c.AccessToken()
	if err != nil {
		return err
	}
	url := "https://api.weixin.qq.com/cgi-bin/message/subscribe/send?access_token=" + token
	body := subscribeMsg{
		Touser:     openid,
		TemplateID: c.cfg.TemplateID,
		Page:       page,
		Data:       data,
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	var resp sendResp
	if err := c.doJSON(req, &resp); err != nil {
		return err
	}
	if resp.ErrCode != 0 {
		// 43101 用户拒收/未订阅，40003 无效 openid
		return fmt.Errorf("wechat: 订阅消息发送失败 errcode=%d errmsg=%s", resp.ErrCode, resp.ErrMsg)
	}
	return nil
}

// ---------- HTTP 工具 ----------

func (c *Client) getJSON(url string, out interface{}) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	return c.doJSON(req, out)
}

func (c *Client) doJSON(req *http.Request, out interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}
