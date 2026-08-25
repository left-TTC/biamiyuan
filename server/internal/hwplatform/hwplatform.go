// hwplatform.go
// 硬件开放平台（健康/养老设备）对接客户端
//
// 对接平台：健康平台（yiyangiot）
//   - 接口根地址：https://health.yiyangiot.com/DeviceMonitor/
//   - 设备(MID/IMEI)在该平台开户并添加后，设备数据（SOS/健康/定位/报警等）会以
//     HTTP 推送（pushType=1/2/3/4/8/10/15...）方式到达对接方配置的「数据上传 url」。
//     本服务通过 /api/v1/hw/push 接收推送（见 internal/api/hwpush.go）。
//   - 查询设备信息（UID/电量/位置）走 deviceLocationQuery.action 接口。
//   - 指令下发走各 action 接口（如开关机/定位/远程监听等，见文档），本包提供通用 Call。
//
// 安全：推送接口不含签名，依赖「数据上传 url」私密性 + HTTPS；生产环境建议在
// 推送 URL 上追加平台账号级密钥（如 /api/v1/hw/push?key=xxx）并在服务端校验。
//
// 模拟模式：未配置 HARDWARE_APPID / HARDWARE_APPKEY 时，VerifyDevice 返回按 IMEI
// 派生的稳定 UID（md5 前 24 位），保证「绑定 → 推送 → 消息路由 → 用户通知」
// 整条链路可在本地联调，无需真实平台账号。
package hwplatform

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL 平台接口根地址（见文档）
const DefaultBaseURL = "https://health.yiyangiot.com/DeviceMonitor/"

// Config 平台接入配置（来自环境变量 HARDWARE_APPID/HARDWARE_APPKEY/HARDWARE_API_BASE）
type Config struct {
	AppID   string // 开户登记时生成
	AppKey  string // 开户登记时生成
	BaseURL string // 接口根地址，默认 DefaultBaseURL
}

// Configured 是否已配置完整平台凭据
func (c Config) Configured() bool {
	return c.AppID != "" && c.AppKey != ""
}

// Client 平台 HTTP 客户端
type Client struct {
	cfg        Config
	httpClient *http.Client
}

// New 创建平台客户端（BaseURL 为空时使用 DefaultBaseURL）
func New(cfg Config) *Client {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Configured 是否已配置完整凭据
func (c *Client) Configured() bool { return c.cfg.Configured() }

// BaseURL 返回接口根地址
func (c *Client) BaseURL() string { return c.cfg.BaseURL }

// AppID 返回开户 appId
func (c *Client) AppID() string { return c.cfg.AppID }

// DeviceInfo 设备查询返回信息（deviceLocationQuery.action → data[0]）
type DeviceInfo struct {
	MID     string  `json:"MID"`     // 设备IMEI
	UID     string  `json:"UID"`     // 设备唯一UID（推送与指令下发的唯一标识）
	Name    string  `json:"Name"`    // 设备昵称
	Battery int     `json:"B"`       // 电量百分比
	Lon     float64 `json:"Lon"`     // 经度
	Lat     float64 `json:"Lat"`     // 纬度
	Pro     string  `json:"Pro"`     // 省份
	City    string  `json:"City"`    // 城市
	Dist    string  `json:"Dist"`    // 县区
	Str     string  `json:"Str"`     // 街道
	Guarder string  `json:"Guarder"` // 监护号码
	Sim     string  `json:"SIM"`     // SIM号
	UT      string  `json:"UT"`      // 定位时间
	Desc    string  `json:"Desc"`    // 描述（如网络中断）
	OnLine  string  `json:"OL"`      // 在线状态（推送通知 Action=-1 时返回）
}

// LocationText 拼接省市区街道（去重，如 海南省→海口市→琼山区→XX路）
func (d *DeviceInfo) LocationText() string {
	var parts []string
	seen := map[string]bool{}
	for _, p := range []string{d.Pro, d.City, d.Dist, d.Str} {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		parts = append(parts, p)
	}
	return strings.Join(parts, "")
}

// baseResp 平台统一应答
type baseResp struct {
	Ret  string            `json:"Ret"`
	Msg  string            `json:"Msg"`
	Data []json.RawMessage `json:"data"`
}

// VerifyDevice 校验设备并获取 UID/电量/位置
//
// 模拟模式（未配置凭据）：按 IMEI 派生稳定 UID，保证本地联调可用。
func (c *Client) VerifyDevice(imei string) (*DeviceInfo, error) {
	imei = strings.TrimSpace(imei)
	if imei == "" {
		return nil, errors.New("设备IMEI为空")
	}
	if !c.cfg.Configured() {
		return demoDeviceInfo(imei), nil
	}
	payload := map[string]interface{}{
		"appId":      c.cfg.AppID,
		"appKey":     c.cfg.AppKey,
		"deviceList": imei,
	}
	var resp baseResp
	if err := c.postJSON("deviceLocationQuery.action", payload, &resp); err != nil {
		return nil, err
	}
	if !strings.EqualFold(strings.TrimSpace(resp.Ret), "Succ") {
		msg := strings.TrimSpace(resp.Msg)
		if msg == "" {
			msg = "未知错误"
		}
		return nil, fmt.Errorf("平台查询失败: %s", msg)
	}
	if len(resp.Data) == 0 {
		return nil, errors.New("平台未返回设备信息，请确认IMEI是否正确且已在该平台添加设备")
	}
	var info DeviceInfo
	if err := json.Unmarshal(resp.Data[0], &info); err != nil {
		return nil, fmt.Errorf("解析设备信息失败: %w", err)
	}
	if info.UID == "" {
		return nil, errors.New("平台未返回设备UID，无法完成绑定")
	}
	return &info, nil
}

// demoDeviceInfo 模拟模式：按 IMEI 派生稳定的 UID（24 位十六进制）
func demoDeviceInfo(imei string) *DeviceInfo {
	sum := md5.Sum([]byte(imei))
	return &DeviceInfo{
		MID:  imei,
		UID:  "uid_" + hex.EncodeToString(sum[:])[:24],
		Name: "智能设备",
		UT:   time.Now().Format("2006-01-02 15:04:05"),
	}
}

// Call 调用平台通用接口（JSON POST，自动携带 appId/appKey）
//
// action 形如 "deviceLocationQuery.action" / "sosCall.action" 等（见文档各指令接口）。
// 模拟模式直接返回成功（不实际下发）。
func (c *Client) Call(action string, payload map[string]interface{}) (map[string]interface{}, error) {
	if !c.cfg.Configured() {
		return map[string]interface{}{"Ret": "Succ", "Msg": "模拟模式：未配置平台凭据，指令已记录不下发"}, nil
	}
	if payload == nil {
		payload = map[string]interface{}{}
	}
	payload["appId"] = c.cfg.AppID
	payload["appKey"] = c.cfg.AppKey
	var resp map[string]interface{}
	if err := c.postJSON(action, payload, &resp); err != nil {
		return nil, err
	}
	if ret, _ := resp["Ret"].(string); !strings.EqualFold(strings.TrimSpace(ret), "Succ") {
		msg, _ := resp["Msg"].(string)
		if msg == "" {
			msg = "未知错误"
		}
		return resp, fmt.Errorf("平台指令下发失败: %s", msg)
	}
	return resp, nil
}

// SendCommand 向设备下发指令
//
//	uid    平台设备唯一UID（绑定后保存在 Device.UID）
//	action 指令接口名（见文档，如 watchSet.action）
//	params 指令参数（除 UID 外的具体字段）
func (c *Client) SendCommand(uid, action string, params map[string]interface{}) (map[string]interface{}, error) {
	if params == nil {
		params = map[string]interface{}{}
	}
	params["UID"] = uid
	return c.Call(action, params)
}

// postJSON 发送 POST JSON 请求并解析应答
func (c *Client) postJSON(action string, payload, out interface{}) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/" + strings.TrimLeft(action, "/")
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("平台请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("平台响应异常 HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
