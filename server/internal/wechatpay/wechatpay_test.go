package wechatpay

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sha256sum(s string) []byte {
	d := sha256.Sum256([]byte(s))
	return d[:]
}

func mustBase64(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// newTestClient 构建配置完整的测试客户端（密钥为本地生成，验证签名/加解密逻辑）
func newTestClient(t *testing.T, wxPriv *rsa.PrivateKey) *Client {
	t.Helper()
	dir := t.TempDir()
	privPath := filepath.Join(dir, "apiclient_key.pem")
	merchantPriv, _ := rsa.GenerateKey(rand.Reader, 2048)
	pem1 := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(merchantPriv)})
	_ = os.WriteFile(privPath, pem1, 0o600)
	pubPath := filepath.Join(dir, "wx_pub.pem")
	pubDER, _ := x509.MarshalPKIXPublicKey(&wxPriv.PublicKey)
	_ = os.WriteFile(pubPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}), 0o600)
	c := New(Config{
		AppID:      "wx_test_appid",
		MchID:      "1900000001",
		APIv3Key:   "0123456789abcdef0123456789abcdef", // 32 字节
		SerialNo:   "SERIALNO123",
		PrivateKey: privPath,
		PublicKey:  pubPath,
		NotifyURL:  "https://example.com/api/v1/pay/notify",
	})
	if !c.Configured() {
		t.Fatal("客户端应配置完成")
	}
	return c
}

func TestConfigured(t *testing.T) {
	c := New(Config{})
	if c.Configured() {
		t.Fatal("空配置不应视为已配置")
	}
}

func TestSignVerify(t *testing.T) {
	wxPriv, _ := rsa.GenerateKey(rand.Reader, 2048)
	c := newTestClient(t, wxPriv)
	msg := "POST\n/v3/pay/transactions/jsapi\n1700000000\nnonce123\n{}\n"
	// 商户签名：用商户公钥（此处未持有）不可直接验证，但可确认签名非空且格式正确
	auth, err := c.signMessage(msg)
	if err != nil {
		t.Fatal(err)
	}
	if auth == "" {
		t.Fatal("签名结果为空")
	}
	// verifyMessage 用微信公钥验签：wxPriv 私钥签的消息应通过
	digest := sha256sum(msg)
	sig, _ := rsa.SignPKCS1v15(rand.Reader, wxPriv, cryptoHashSHA256, digest)
	good := base64.StdEncoding.EncodeToString(sig)
	if !c.verifyMessage(good, msg) {
		t.Fatal("verifyMessage 应验证通过")
	}
	if c.verifyMessage(base64.StdEncoding.EncodeToString([]byte("bad")), msg) {
		t.Fatal("伪造签名不应通过")
	}
}

func TestAESGCMRoundtrip(t *testing.T) {
	wxPriv, _ := rsa.GenerateKey(rand.Reader, 2048)
	c := newTestClient(t, wxPriv)
	key := []byte(c.cfg.APIv3Key)
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	_, _ = rand.Read(nonce)
	plain := []byte(`{"out_trade_no":"O123","trade_state":"SUCCESS"}`)
	sealed := gcm.Seal(nil, nonce, plain, []byte("transaction"))
	enc := base64.StdEncoding.EncodeToString(sealed)
	dec, err := c.aesGCMDecrypt(enc, string(nonce), "transaction")
	if err != nil {
		t.Fatal(err)
	}
	if string(dec) != string(plain) {
		t.Fatal("AES-GCM 解密结果不一致")
	}
}

func TestRSAPublicEncryptDecrypt(t *testing.T) {
	wxPriv, _ := rsa.GenerateKey(rand.Reader, 2048)
	c := newTestClient(t, wxPriv)
	plain := []byte(`{"appid":"wx","total_amount":100}`)
	enc, err := c.rsaEncryptOAEP(plain)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.StdEncoding.DecodeString(enc)
	dec, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, wxPriv, raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(dec) != string(plain) {
		t.Fatal("RSA-OAEP 解密结果不一致")
	}
}

func TestVerifyAndDecryptNotify(t *testing.T) {
	wxPriv, _ := rsa.GenerateKey(rand.Reader, 2048)
	c := newTestClient(t, wxPriv)

	// 构造微信回调：先用 APIv3 密钥 AES-GCM 加密 resource
	payload := map[string]interface{}{
		"out_trade_no":   "O20260824001",
		"transaction_id": "4200000000001",
		"trade_state":    "SUCCESS",
		"success_time":   "2026-08-24T10:00:00+08:00",
	}
	raw, _ := json.Marshal(payload)
	key := []byte(c.cfg.APIv3Key)
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	nonce := []byte("abcdefghijkl") // 12 字节 ASCII（JSON 安全）
	sealed := gcm.Seal(nil, nonce, raw, []byte("transaction"))
	env := map[string]interface{}{
		"id":            "evt_1",
		"event_type":    "TRANSACTION.SUCCESS",
		"resource_type": "encrypt-resource",
		"resource": map[string]interface{}{
			"algorithm":       "AEAD_AES_256_GCM",
			"ciphertext":      base64.StdEncoding.EncodeToString(sealed),
			"associated_data": "transaction",
			"nonce":           string(nonce),
			"original_type":   "transaction",
		},
	}
	body, _ := json.Marshal(env)
	// 微信用其私钥签名：timestamp\nnonce\nbody\n
	ts, n := "1700000000", "wxnonce"
	msg := ts + "\n" + n + "\n" + string(body) + "\n"
	digest := sha256sum(msg)
	sig, _ := rsa.SignPKCS1v15(rand.Reader, wxPriv, cryptoHashSHA256, digest)
	signature := base64.StdEncoding.EncodeToString(sig)

	got, err := c.VerifyAndDecryptNotify(ts, n, signature, string(body))
	if err != nil {
		t.Fatal(err)
	}
	if got.OutTradeNo != "O20260824001" || got.TransactionID != "4200000000001" {
		t.Fatalf("回调解析错误: %+v", got)
	}
	// 篡改 body → 验签失败
	tampered := strings.Replace(string(body), "evt_1", "evt_2", 1)
	if _, err := c.VerifyAndDecryptNotify(ts, n, signature, tampered); err == nil {
		t.Fatal("篡改回调应验签失败")
	}
}

func TestPaymentParams(t *testing.T) {
	wxPriv, _ := rsa.GenerateKey(rand.Reader, 2048)
	c := newTestClient(t, wxPriv)
	p := c.PaymentParams("prepay_id_123")
	if p["package"] != "prepay_id=prepay_id_123" {
		t.Fatalf("package 错误: %v", p)
	}
	if p["signType"] != "RSA" || p["timeStamp"] == "" || p["nonceStr"] == "" || p["paySign"] == "" {
		t.Fatalf("支付参数不完整: %v", p)
	}
	// 校验 paySign = HMAC-SHA256(apiV3Key, "appId\ntimeStamp\nnonceStr\npackage\n")
	expect := c.hmacSHA256(c.cfg.AppID + "\n" + p["timeStamp"] + "\n" + p["nonceStr"] + "\n" + p["package"] + "\n")
	if p["paySign"] != expect {
		t.Fatal("paySign 校验失败")
	}
}

