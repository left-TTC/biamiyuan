// wechatpay.go
// 微信支付 API v3 客户端
//
// 架构：小程序 → 服务器（本包） → 微信支付（HTTPS API）
// 实现能力：
//   - JSAPI 统一下单（支付）→ prepay_id → 小程序 wx.requestPayment
//   - 支付结果回调验签 + 解密（AES-256-GCM）
//   - 订单查询 / 退款
//   - 商家转账到零钱（提现打款，v3 公钥加密请求体）
//
// 配置项（见 Config / .env.example）：
//   WECHAT_PAY_MCH_ID            微信支付商户号
//   WECHAT_PAY_MCH_KEY           APIv3 密钥（32 位，解密回调与构造支付签名）
//   WECHAT_PAY_SERIAL_NO         商户 API 证书序列号
//   WECHAT_PAY_PRIVATE_KEY_PATH  商户 API 私钥 apiclient_key.pem 路径
//   WECHAT_PAY_PUBLIC_KEY_PATH   微信支付公钥（商户平台下载）PEM 路径
//   WECHAT_PAY_NOTIFY_URL        支付结果回调地址（需公网可达 HTTPS）
//
// 未配置完整凭据时 Configured() 返回 false，上层回退「模拟支付/提现」模式，
// 保证演示环境可运行；配置后自动切换为真实微信支付。
package wechatpay

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	apiBase = "https://api.mch.weixin.qq.com"
	// cryptoHashSHA256 统一使用 SHA256 哈希（签名用）
	cryptoHashSHA256 = crypto.SHA256
)

// ErrNotConfigured 未配置完整微信支付凭据
var ErrNotConfigured = errors.New("wechatpay: 未配置商户号/APIv3密钥/证书，无法调用真实微信支付")

// Config 微信支付配置（全部来自环境变量）
type Config struct {
	AppID      string // 小程序 AppID（WECHAT_APPID）
	MchID      string // 商户号
	APIv3Key   string // APIv3 密钥（32 字节）
	SerialNo   string // 商户 API 证书序列号
	PrivateKey string // 商户 API 私钥（PEM 文件路径）
	PublicKey  string // 微信支付公钥（PEM 文件路径，转账加密 + 回调验签）
	NotifyURL  string // 支付结果回调地址
}

// Configured 是否配置齐全可调用真实接口
func (c Config) Configured() bool {
	return c.AppID != "" && c.MchID != "" && c.APIv3Key != "" && c.SerialNo != "" && c.PrivateKey != "" && c.PublicKey != ""
}

// Client 微信支付客户端
type Client struct {
	cfg        Config
	httpClient *http.Client
	privKey    *rsa.PrivateKey // 商户 API 私钥（签名 / 退款）
	wxPubKey   *rsa.PublicKey  // 微信支付公钥（回调验签 / 转账请求体加密）
}

// New 创建微信支付客户端；未配置时返回可用客户端但 Configured()=false
func New(cfg Config) *Client {
	c := &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	priv, err := loadPrivateKey(cfg.PrivateKey)
	if err == nil {
		c.privKey = priv
	}
	pub, err := loadPublicKey(cfg.PublicKey)
	if err == nil {
		c.wxPubKey = pub
	}
	return c
}

// Configured 是否可调用真实微信支付
func (c *Client) Configured() bool { return c.cfg.Configured() && c.privKey != nil && c.wxPubKey != nil }

// MchID 返回商户号（日志展示用）
func (c *Client) MchID() string { return c.cfg.MchID }

// ---------- 密钥加载 ----------

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	if path == "" {
		return nil, errors.New("商户私钥路径为空")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, errors.New("商户私钥 PEM 解析失败")
	}
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rk, ok := k.(*rsa.PrivateKey); ok {
			return rk, nil
		}
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func loadPublicKey(path string) (*rsa.PublicKey, error) {
	if path == "" {
		return nil, errors.New("微信支付公钥路径为空")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, errors.New("微信支付公钥 PEM 解析失败")
	}
	if k, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if rk, ok := k.(*rsa.PublicKey); ok {
			return rk, nil
		}
	}
	if k, err := x509.ParseCertificate(block.Bytes); err == nil {
		if pk, ok := k.PublicKey.(*rsa.PublicKey); ok {
			return pk, nil
		}
	}
	return nil, errors.New("微信支付公钥解析失败")
}

// ---------- 签名 / 加解密 ----------

// signMessage 用商户私钥对消息做 SHA256-RSA 签名，返回 base64
func (c *Client) signMessage(message string) (string, error) {
	digest := sha256.Sum256([]byte(message))
	sig, err := rsa.SignPKCS1v15(rand.Reader, c.privKey, cryptoHashSHA256, digest[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// hmacSHA256 用 APIv3 密钥做 HMAC-SHA256（构造 wx.requestPayment paySign）
func (c *Client) hmacSHA256(message string) string {
	mac := hmac.New(sha256.New, []byte(c.cfg.APIv3Key))
	mac.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// aesGCMDecrypt 用 APIv3 密钥解密回调/响应的 resource 密文
func (c *Client) aesGCMDecrypt(ciphertext, nonce, aad string) ([]byte, error) {
	key := []byte(c.cfg.APIv3Key)
	if len(key) != 32 {
		return nil, fmt.Errorf("APIv3 密钥长度必须为 32 字节，当前 %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, []byte(nonce), raw, []byte(aad))
}

// rsaEncryptOAEP 用微信支付公钥加密请求体（转账到零钱要求）
func (c *Client) rsaEncryptOAEP(plain []byte) (string, error) {
	label := []byte("")
	enc, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, c.wxPubKey, plain, label)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(enc), nil
}

// verifyMessage 用微信支付公钥校验签名
func (c *Client) verifyMessage(signature, message string) bool {
	sig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return false
	}
	digest := sha256.Sum256([]byte(message))
	return rsa.VerifyPKCS1v15(c.wxPubKey, cryptoHashSHA256, digest[:], sig) == nil
}

// authHeader 构造 v3 请求头 Authorization
func (c *Client) authHeader(method, path, body string) (string, error) {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	nonce := randomHex(16)
	msg := method + "\n" + path + "\n" + ts + "\n" + nonce + "\n" + body + "\n"
	sig, err := c.signMessage(msg)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		`WECHATPAY2-SHA256-RSA mchid="%s",nonce_str="%s",timestamp="%s",serial_no="%s",signature="%s"`,
		c.cfg.MchID, nonce, ts, c.cfg.SerialNo, sig), nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// ---------- HTTP 请求 ----------

// postJSON 发起 v3 签名请求。encryptBody=true 时请求体先用微信支付公钥 RSA-OAEP 加密（转账）。
// 返回原始响应字节（部分接口响应为密文，需调用方解密）。
func (c *Client) postJSON(method, path string, body interface{}, encryptBody bool) ([]byte, int, error) {
	var raw []byte
	var err error
	if body != nil {
		raw, err = json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
	}
	sendBody := raw
	if encryptBody {
		enc, err := c.rsaEncryptOAEP(raw)
		if err != nil {
			return nil, 0, err
		}
		sendBody = []byte(enc)
	}
	url := apiBase + path
	req, err := http.NewRequest(method, url, bytes.NewReader(sendBody))
	if err != nil {
		return nil, 0, err
	}
	auth, err := c.authHeader(method, path, string(sendBody))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "wechatpay-go/mall-server")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode >= 400 {
		return out, resp.StatusCode, fmt.Errorf("微信支付接口返回 %d: %s", resp.StatusCode, string(out))
	}
	return out, resp.StatusCode, nil
}

// ---------- JSAPI 统一下单（支付） ----------

type amountDTO struct {
	Total    int64  `json:"total"`
	Currency string `json:"currency"`
	Refund   int64  `json:"refund,omitempty"`
}

type payerDTO struct {
	OpenID string `json:"openid"`
}

type jsapiPrepayReq struct {
	AppID       string     `json:"appid"`
	MchID       string     `json:"mchid"`
	Description string     `json:"description"`
	OutTradeNo  string     `json:"out_trade_no"`
	NotifyURL   string     `json:"notify_url"`
	Amount      *amountDTO `json:"amount"`
	Payer       *payerDTO  `json:"payer"`
}

type jsapiPrepayResp struct {
	PrepayID string `json:"prepay_id"`
}

// JSAPIPrepay 微信支付统一下单（JSAPI 拉起小程序支付）。
// 返回 prepay_id；amountFen 为金额（分）。
func (c *Client) JSAPIPrepay(openid, outTradeNo, description string, amountFen int64) (string, error) {
	if !c.Configured() {
		return "", ErrNotConfigured
	}
	body := jsapiPrepayReq{
		AppID:       c.cfg.AppID,
		MchID:       c.cfg.MchID,
		Description: truncateRune(description, 127),
		OutTradeNo:  outTradeNo,
		NotifyURL:   c.cfg.NotifyURL,
		Amount:      &amountDTO{Total: amountFen, Currency: "CNY"},
		Payer:       &payerDTO{OpenID: openid},
	}
	raw, status, err := c.postJSON(http.MethodPost, "/v3/pay/transactions/jsapi", body, false)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("统一下单失败 HTTP %d: %s", status, string(raw))
	}
	var resp jsapiPrepayResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", err
	}
	if resp.PrepayID == "" {
		return "", fmt.Errorf("统一下单返回缺少 prepay_id: %s", string(raw))
	}
	return resp.PrepayID, nil
}

// PaymentParams 构造小程序 wx.requestPayment 所需参数
func (c *Client) PaymentParams(prepayID string) map[string]string {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	nonce := randomHex(16)
	pkg := "prepay_id=" + prepayID
	msg := c.cfg.AppID + "\n" + ts + "\n" + nonce + "\n" + pkg + "\n"
	return map[string]string{
		"timeStamp": ts,
		"nonceStr":  nonce,
		"package":   pkg,
		"signType":  "RSA",
		"paySign":   c.hmacSHA256(msg),
	}
}

// QueryOrder 查询订单支付结果。返回 (tradeState, transactionID, err)。
// tradeState: SUCCESS / REFUND / NOTPAY / CLOSED / REVOKED / USERPAYING / PAYERROR
func (c *Client) QueryOrder(outTradeNo string) (string, string, error) {
	if !c.Configured() {
		return "", "", ErrNotConfigured
	}
	path := "/v3/pay/transactions/out-trade-no/" + outTradeNo + "?mchid=" + c.cfg.MchID
	raw, status, err := c.postJSON(http.MethodGet, path, nil, false)
	if err != nil {
		return "", "", err
	}
	if status != http.StatusOK {
		return "", "", fmt.Errorf("订单查询失败 HTTP %d: %s", status, string(raw))
	}
	var resp struct {
		TradeState    string `json:"trade_state"`
		TransactionID string `json:"transaction_id"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", "", err
	}
	return resp.TradeState, resp.TransactionID, nil
}

// ---------- 退款 ----------

type refundReq struct {
	OutTradeNo  string     `json:"out_trade_no"`
	OutRefundNo string     `json:"out_refund_no"`
	Amount      *amountDTO `json:"amount"`
}

// Refund 退款（订单支付后退款）。refundFen / totalFen 为分。
func (c *Client) Refund(outTradeNo, outRefundNo string, refundFen, totalFen int64) error {
	if !c.Configured() {
		return ErrNotConfigured
	}
	body := refundReq{
		OutTradeNo:  outTradeNo,
		OutRefundNo: outRefundNo,
		Amount:      &amountDTO{Refund: refundFen, Total: totalFen, Currency: "CNY"},
	}
	raw, status, err := c.postJSON(http.MethodPost, "/v3/refund/domestic/refunds", body, false)
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusAccepted {
		return fmt.Errorf("退款申请失败 HTTP %d: %s", status, string(raw))
	}
	return nil
}

// ---------- 商家转账到零钱（提现打款） ----------

type transferDetailDTO struct {
	OutDetailNo    string `json:"out_detail_no"`
	TransferAmount int64  `json:"transfer_amount"`
	TransferRemark string `json:"transfer_remark"`
	OpenID         string `json:"openid"`
}

type transferReq struct {
	AppID              string              `json:"appid"`
	OutBatchNo         string              `json:"out_batch_no"`
	BatchName          string              `json:"batch_name"`
	BatchRemark        string              `json:"batch_remark"`
	TotalAmount        int64               `json:"total_amount"`
	TotalNum           int                 `json:"total_num"`
	TransferDetailList []transferDetailDTO `json:"transfer_detail_list"`
}

// TransferToBalance 商家转账到零钱（用户提现打款）。
// openid 为收款用户微信 openid，amountFen 金额（分，最低 0.3 元）。
// 返回微信批次号 batch_id（转账异步到账，受理成功即返回）。
func (c *Client) TransferToBalance(openid, outBatchNo, outDetailNo, remark string, amountFen int64) (string, error) {
	if !c.Configured() {
		return "", ErrNotConfigured
	}
	body := transferReq{
		AppID:       c.cfg.AppID,
		OutBatchNo:  outBatchNo,
		BatchName:   "用户提现",
		BatchRemark: "安全监护商城提现打款",
		TotalAmount: amountFen,
		TotalNum:    1,
		TransferDetailList: []transferDetailDTO{{
			OutDetailNo:    outDetailNo,
			TransferAmount: amountFen,
			TransferRemark: truncateRune(remark, 32),
			OpenID:         openid,
		}},
	}
	raw, status, err := c.postJSON(http.MethodPost, "/v3/transfer/batches", body, true)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK && status != http.StatusAccepted {
		return "", fmt.Errorf("转账失败 HTTP %d: %s", status, string(raw))
	}
	// 响应 resource 为 AES-GCM 密文，需解密
	var enc struct {
		Resource struct {
			Ciphertext string `json:"ciphertext"`
			Nonce      string `json:"nonce"`
			AAD        string `json:"associated_data"`
		} `json:"resource"`
	}
	if err := json.Unmarshal(raw, &enc); err != nil {
		return "", err
	}
	dec, err := c.aesGCMDecrypt(enc.Resource.Ciphertext, enc.Resource.Nonce, enc.Resource.AAD)
	if err != nil {
		return "", fmt.Errorf("转账响应解密失败: %v", err)
	}
	var resp struct {
		OutBatchNo  string `json:"out_batch_no"`
		BatchID     string `json:"batch_id"`
		State       string `json:"state"`
		BatchStatus string `json:"batch_status"`
	}
	if err := json.Unmarshal(dec, &resp); err != nil {
		return "", err
	}
	if resp.BatchStatus == "REJECTED" || resp.BatchStatus == "CLOSED" || resp.BatchStatus == "FINISHED_FAIL" {
		return resp.BatchID, fmt.Errorf("微信转账失败 state=%s", resp.BatchStatus)
	}
	return resp.BatchID, nil
}

// ---------- 支付结果回调 ----------

// NotifyPayload 支付回调解密后的内容
type NotifyPayload struct {
	OutTradeNo    string `json:"out_trade_no"`
	TransactionID string `json:"transaction_id"`
	TradeState    string `json:"trade_state"`
	Payer         struct {
		OpenID string `json:"openid"`
	} `json:"payer"`
	Amount struct {
		Total int64 `json:"total"`
	} `json:"amount"`
	SuccessTime string `json:"success_time"`
}

// VerifyAndDecryptNotify 校验回调签名并解密 resource。
// 签名校验失败返回错误；成功返回解密后的支付结果。
func (c *Client) VerifyAndDecryptNotify(timestamp, nonce, signature, body string) (*NotifyPayload, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	msg := timestamp + "\n" + nonce + "\n" + body + "\n"
	if !c.verifyMessage(signature, msg) {
		return nil, errors.New("支付回调签名校验失败")
	}
	var envelop struct {
		EventType    string `json:"event_type"`
		ResourceType string `json:"resource_type"`
		Resource     struct {
			Algorithm      string `json:"algorithm"`
			Ciphertext     string `json:"ciphertext"`
			AssociatedData string `json:"associated_data"`
			Nonce          string `json:"nonce"`
		} `json:"resource"`
	}
	if err := json.Unmarshal([]byte(body), &envelop); err != nil {
		return nil, err
	}
	if envelop.ResourceType != "encrypt-resource" {
		return nil, fmt.Errorf("非预期回调 resource_type=%s", envelop.ResourceType)
	}
	dec, err := c.aesGCMDecrypt(envelop.Resource.Ciphertext, envelop.Resource.Nonce, envelop.Resource.AssociatedData)
	if err != nil {
		return nil, fmt.Errorf("回调解密失败: %v", err)
	}
	var payload NotifyPayload
	if err := json.Unmarshal(dec, &payload); err != nil {
		return nil, err
	}
	if payload.OutTradeNo == "" {
		return nil, errors.New("回调数据缺少 out_trade_no")
	}
	return &payload, nil
}

// truncateRune 按字符数截断字符串
func truncateRune(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}


