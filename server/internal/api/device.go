// device.go
// 设备与消息接口
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"socialserver/internal/store"
)

// deviceTypes 设备类型（与小程序端 utils/store.js DEVICE_TYPES 保持一致）
var deviceTypes = map[string]string{
	"camera":   "智能摄像头",
	"smoke":    "烟雾报警器",
	"gas":      "燃气报警器",
	"sos":      "一键求助",
	"fall":     "跌倒监测手环",
	"watch":    "定位手表",
	"lock":     "智能门锁",
	"infrared": "红外感应器",
	"water":    "水浸传感器",
}

// alarmMap 各类型设备的报警文案
var alarmMap = map[string][2]string{
	"camera":   {"画面异常报警", "摄像头画面长时间无变化，请检查是否被遮挡！"},
	"smoke":    {"烟雾浓度过高", "检测到烟雾浓度超标，可能发生火情，请立即检查！"},
	"gas":      {"燃气泄漏报警", "检测到燃气浓度异常，请立即关闭阀门并开窗通风！"},
	"sos":      {"紧急求助", "有人按下一键求助按钮，请立即确认安全！"},
	"fall":     {"跌倒检测", "检测到佩戴者疑似跌倒，请尽快查看！"},
	"watch":    {"电子围栏越界", "定位手表已离开安全区域，请确认佩戴者位置！"},
	"lock":     {"门锁异常报警", "检测到门锁被暴力撬动，请立即确认！"},
	"infrared": {"红外入侵报警", "检测到红外人体移动，请查看实时画面！"},
	"water":    {"水浸报警", "检测到漏水，请检查水管阀门！"},
}

type addDeviceReq struct {
	Name string `json:"name"`
	Type string `json:"type"`
	SN   string `json:"sn"`
}

// GET /api/v1/devices 设备列表
func (a *API) listDevices(w http.ResponseWriter, r *http.Request) {
	ok(w, a.store.ListDevices(userFrom(r).ID))
}

// POST /api/v1/devices 添加设备
func (a *API) addDevice(w http.ResponseWriter, r *http.Request) {
	var req addDeviceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.SN = strings.TrimSpace(req.SN)
	if req.Name == "" || req.SN == "" {
		fail(w, http.StatusBadRequest, "设备名称与序列号不能为空")
		return
	}
	if tooLong(req.Name, maxDeviceName) {
		fail(w, http.StatusBadRequest, "设备名称最长 30 个字符")
		return
	}
	if tooLong(req.SN, maxSN) {
		fail(w, http.StatusBadRequest, "设备序列号最长 50 个字符")
		return
	}
	if _, ok := deviceTypes[req.Type]; !ok {
		fail(w, http.StatusBadRequest, "不支持的设备类型")
		return
	}
	d := a.store.AddDevice(userFrom(r).ID, &store.Device{
		Name:          req.Name,
		Type:          req.Type,
		SN:            req.SN,
		NotifyEnabled: true,
	})
	a.store.AddMessage(d.ID, "info", "设备已绑定", "「"+d.Name+"」绑定成功，开始守护您的家")
	ok(w, d)
}

// DELETE /api/v1/devices/{id} 移除设备
func (a *API) removeDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !a.store.RemoveDevice(userFrom(r).ID, id) {
		fail(w, http.StatusNotFound, "设备不存在")
		return
	}
	ok(w, map[string]string{"id": id})
}

// POST /api/v1/devices/{id}/alarm 设备报警上报
//
// 真实环境：设备端/网关通过硬件平台推送（/api/v1/hw/push）上报报警；
// 本接口用于小程序/管理端手动触发报警（演示），同样走「站内消息 + 微信订阅消息推送」。
func (a *API) alarmDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	d := a.store.GetDevice(id)
	if d == nil {
		fail(w, http.StatusNotFound, "设备不存在")
		return
	}
	a.store.UpdateDeviceStatus(id, "alarm")
	am, exists := alarmMap[d.Type]
	if !exists {
		am = [2]string{"设备报警", "设备检测到异常情况，请及时处理"}
	}
	msg := a.store.AddMessage(id, "alarm", am[0], am[1])
	// 微信订阅消息推送（关闭小程序也能收到通知）
	push := a.pushDeviceEvent(a.store.UserByID(d.UserID), am[0], am[1])

	ok(w, map[string]interface{}{
		"device":  a.store.GetDevice(id),
		"message": msg,
		"push":    push,
	})
}

// GET /api/v1/messages 设备消息列表
func (a *API) listMessages(w http.ResponseWriter, r *http.Request) {
	ok(w, a.store.ListMessages(userFrom(r).ID))
}

// POST /api/v1/messages/{id}/read 标记消息已读
func (a *API) markMessageRead(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !a.store.MarkRead(id) {
		fail(w, http.StatusNotFound, "消息不存在")
		return
	}
	ok(w, map[string]string{"id": id})
}

// GET /api/v1/notify/template-id 获取订阅消息模板 ID
func (a *API) notifyTemplate(w http.ResponseWriter, r *http.Request) {
	tmpl := ""
	if a.wechat != nil {
		tmpl = a.wechat.TemplateID()
	}
	ok(w, map[string]string{"templateId": tmpl})
}

// POST /api/v1/notify/subscribe 记录订阅授权
//
// 小程序端 wx.requestSubscribeMessage 成功后调用（一次性订阅，每次授权可推送 1 次）
func (a *API) subscribe(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r)
	a.store.AddSubscribeQuota(user.ID, 1)
	ok(w, map[string]string{"status": "subscribed"})
}

// alarmAllReq 统一发送警告请求
type alarmAllReq struct {
	DeviceCount int `json:"deviceCount"` // 小程序端本地设备数（服务器未同步设备时用于正确统计）
}

// POST /api/v1/devices/alarm-all 向所有设备发送警告（测试按钮）
//
// 1. 将服务器上用户所有设备标记为报警并生成警告消息
// 2. 尝试通过微信订阅消息推送给用户（关闭小程序也能收到通知）
func (a *API) alarmAll(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r)
	devices := a.store.ListDevices(user.ID)

	var req alarmAllReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if req.DeviceCount < 0 || req.DeviceCount > 1000 {
		fail(w, http.StatusBadRequest, "设备数异常")
		return
	}

	count := len(devices)
	if req.DeviceCount > count {
		count = req.DeviceCount
	}
	if count == 0 {
		a.store.AddMessage("system", "alarm", "系统安全警告", "收到全局安全测试指令，请检查各设备状态")
	} else {
		for _, d := range devices {
			a.store.UpdateDeviceStatus(d.ID, "alarm")
			am, ok := alarmMap[d.Type]
			if !ok {
				am = [2]string{"设备报警", "设备检测到异常情况，请及时处理"}
			}
			a.store.AddMessage(d.ID, "alarm", am[0], am[1])
		}
	}

	push := a.pushAlarmNotify(user, count)
	ok(w, map[string]interface{}{
		"count": count,
		"push":  push,
	})
}

// pushAlarmNotify 通过微信订阅消息推送报警通知
//
// 返回推送结果说明：
//   - mode=simulate   未配置微信凭据，模拟推送（真实推送需配置 WECHAT_*）
//   - mode=no_openid  用户未在小程序内完成 wx-login 绑定
//   - mode=no_quota   订阅次数不足（一次性订阅，需重新授权）
//   - mode=sent       微信订阅消息已成功推送
func (a *API) pushAlarmNotify(user *store.User, count int) map[string]string {
	if a.wechat == nil || !a.wechat.Configured() {
		return map[string]string{
			"mode": "simulate",
			"msg":  "未配置 WECHAT_APPID/SECRET/TEMPLATE_ID，本次为模拟推送（已生成警告消息）",
		}
	}
	if user.OpenID == "" {
		return map[string]string{
			"mode": "no_openid",
			"msg":  "未绑定微信 openid，请在小程序内完成微信授权（wx-login）后重试",
		}
	}
	if !a.store.ConsumeSubscribe(user.ID) {
		return map[string]string{
			"mode": "no_quota",
			"msg":  "订阅次数不足，请在小程序内重新点击「开启报警通知」授权订阅",
		}
	}
	// 模板字段（thing1/time2）按申请模板时的字段调整
	err := a.wechat.SendSubscribeMessage(user.OpenID, "pages/index/index", map[string]interface{}{
		"thing1": map[string]string{"value": "设备安全警报"},
		"thing2": map[string]string{"value": time2Text(count)},
	})
	if err != nil {
		return map[string]string{"mode": "error", "msg": err.Error()}
	}
	return map[string]string{"mode": "sent", "msg": "微信订阅消息已推送，请查看微信服务通知"}
}

func time2Text(count int) string {
	now := time.Now()
	return fmt.Sprintf("%02d时%02d分 %d台设备报警", now.Hour(), now.Minute(), count)
}
