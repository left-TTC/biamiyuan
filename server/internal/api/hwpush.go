// hwpush.go
// 硬件开放平台对接：设备绑定 + 平台数据推送接收（webhook）+ 消息路由到用户
//
// 数据流：
//
//	硬件设备 → 健康平台(解析) → HTTP推送(/api/v1/hw/push) → 按 UID/MID 找到绑定设备
//	→ 归属用户 → 站内消息 + 微信订阅消息推送（关闭小程序也能收到）
//
// 平台推送字段全集见对接文档；实际推送按业务场景返回字段子集。
package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"socialserver/internal/hwplatform"
	"socialserver/internal/store"
)

// hwPushPayload 平台推送统一样式（字段全集）
type hwPushPayload struct {
	PushType  int     `json:"pushType"` // 1 SOS / 2 健康 / 3 定位 / 4 通知 / 8 微聊 / 10 睡眠 / 15 设备数据
	MID       string  `json:"MID"`      // 设备IMEI
	UID       string  `json:"UID"`      // 设备唯一UID
	Name      string  `json:"Name"`     // 设备昵称
	Time      string  `json:"Time"`     // 事件时间
	Content   string  `json:"content"`  // 微聊内容（文字/语音URL）
	MsgType   int     `json:"msgType"`  // 微聊消息类型 1文字 2语音
	Guarder   string  `json:"Guarder"`  // 监护号码
	Action    int     `json:"Action"`   // 通知类型（pushType=4）
	SMID      string  `json:"SMID"`     // 消息ID
	Lon       float64 `json:"Lon"`
	Lat       float64 `json:"Lat"`
	Radius    int     `json:"Radius"`
	Pro       string  `json:"Pro"`
	City      string  `json:"City"`
	Dist      string  `json:"Dist"`
	Str       string  `json:"Str"`
	B         int     `json:"B"`       // 电量
	OL        any     `json:"OL"`      // 在线状态（0离线 1在线）
	DevType   string  `json:"DevType"` // 门磁等设备类型
	PDType    string  `json:"PDType"`  // NB产品类型
	MCStatus  string  `json:"MCStatus"`
	EventType string  `json:"EventType"`
	// 健康数据（pushType=2）
	Type    int     `json:"Type"` // 1心率 2血压 3血氧 4体温 5血糖 6尿酸 7HRV 8呼吸 9血脂
	H       int     `json:"H"`    // 心率
	O       int     `json:"O"`    // 血氧
	W       float64 `json:"W"`    // 体温
	X       int     `json:"X"`    // 血压高压
	Y       int     `json:"Y"`    // 血压低压
	G       float64 `json:"G"`    // 血糖
	URAC    float64 `json:"URAC"` // 尿酸
	SIGHRV  int     `json:"SIGHRV"`
	Breathe int     `json:"Breathe"`
	Bli     float64 `json:"Bli"` // 血脂
	// 设备数据（pushType=15）
	DType int `json:"dType"` // 1 JSZN睡眠雷达 2 JSZN跌倒雷达 3 温湿度 4 4G睡眠带
}

// hwAlarmInfo 通知推送（pushType=4）Action → 文案 + 是否报警
type hwAlarmInfo struct {
	title   string
	content string
	alarm   bool
}

// hwActionMap 平台通知类型 Action → 站内消息文案（对照对接文档 4.通知推送）
var hwActionMap = map[int]hwAlarmInfo{
	-1:  {"设备状态变化", "设备在线状态已更新（0 离线 / 1 在线）", false},
	4:   {"进入围栏停留", "设备已停留在安全围栏内", false},
	5:   {"离开安全围栏", "设备已离开安全围栏，请及时确认位置！", true},
	6:   {"进入安全围栏", "设备已回到安全围栏内", false},
	7:   {"SOS紧急求助", "设备发出SOS紧急求助，请立即确认安全！", true},
	9:   {"设备电量过低", "设备电量过低，请及时充电以免失联！", true},
	10:  {"设备被摘除", "检测到设备被摘除/佩戴状态异常，请确认！", true},
	11:  {"跌倒报警", "检测到佩戴者疑似跌倒，请尽快查看！", true},
	22:  {"低温报警", "检测到环境温度过低，请及时保暖！", true},
	23:  {"高温报警", "检测到环境温度过高，请及时降温！", true},
	24:  {"更换SIM卡", "设备SIM卡已更换", false},
	26:  {"WiFi已断开", "设备WiFi连接已断开", false},
	27:  {"WiFi已连接", "设备WiFi连接已恢复", false},
	28:  {"WiFi离线", "设备WiFi离线，可能无法正常通信", true},
	35:  {"社区设备报警", "社区（居家）养老设备触发报警，请及时处理！", true},
	36:  {"防盗设备报警", "防盗设备触发报警，请立即确认！", true},
	37:  {"状态异常告警", "设备状态异常，请检查设备！", true},
	42:  {"八件套布防", "八件套安防已布防", false},
	43:  {"八件套撤防", "八件套安防已撤防", false},
	44:  {"八件套在家布防", "八件套安防已进入在家布防模式", false},
	45:  {"八件套报警", "八件套安防触发报警，请立即处理！", true},
	47:  {"设备WiFi不一致", "设备WiFi配置与平台不一致，请检查", true},
	49:  {"红外报警", "检测到红外人体活动，请查看现场情况！", true},
	50:  {"NB按键报警", "NB设备按键触发报警！", true},
	51:  {"NB防拆报警", "NB设备被拆卸，请确认！", true},
	52:  {"NB设备恢复", "NB设备已恢复正常", false},
	61:  {"NB设备报警", "NB设备触发报警！", true},
	63:  {"人体存在报警", "检测到人体存在，请注意！", true},
	67:  {"NB设备测试", "NB设备测试报警（测试消息）", false},
	84:  {"八件套网关离线", "八件套网关已离线", false},
	85:  {"八件套网关上线", "八件套网关已上线", false},
	86:  {"八件套添加子设备", "八件套已添加子设备", false},
	87:  {"八件套删除子设备", "八件套已删除子设备", false},
	113: {"门磁事件上报", "门磁传感器事件上报", false},
	114: {"烟感/气感/门磁事件", "烟感、气感或门磁设备触发事件，请检查！", true},
	115: {"拉绳SOS上报", "拉绳SOS设备触发紧急求助！", true},
	116: {"SCA设备事件", "SCA设备事件上报，请确认！", true},
	117: {"4G视频门磁事件", "4G视频门磁事件上报", false},
	118: {"防跌倒雷达报警", "防跌倒雷达检测到跌倒，请尽快查看！", true},
	119: {"网关子设备报警", "D5网关子设备触发报警！", true},
	121: {"智能胸牌告警", "智能胸牌设备告警，请确认！", true},
	122: {"NB温湿度报警", "温湿度设备触发报警！", true},
	123: {"燃气泄漏报警", "检测到燃气泄漏，请立即关阀开窗！", true},
	124: {"烟雾报警", "检测到烟雾浓度超标，请立即检查火情！", true},
	125: {"水浸报警", "检测到漏水，请检查水管阀门！", true},
	126: {"摄像头报警", "摄像头检测到异常，请查看画面！", true},
	127: {"JSZN跌倒报警", "跌倒监测设备检测到跌倒，请尽快查看！", true},
	128: {"JSZN井盖报警", "井盖位移/异常报警，请检查！", true},
	129: {"JSZN燃气报警", "燃气浓度异常，请立即检查！", true},
	130: {"JSZN红外报警", "红外探测到入侵，请查看！", true},
	131: {"对讲SOS报警", "对讲设备触发SOS求助！", true},
	132: {"ZML_SOS报警", "SOS设备触发紧急求助！", true},
	134: {"AI智能告警", "AI智能报警器触发告警！", true},
	135: {"YJ设备事件", "YJ设备事件上报", false},
	138: {"YL睡眠雷达", "睡眠雷达事件上报", false},
	139: {"JHL门磁报警", "JHL门磁报警器触发报警！", true},
	153: {"STR设备事件", "STR设备红外报警！", true},
	200: {"设备信息更新", "设备昵称、分组或签名等资料已更新", false},
}

// asBool 宽松解析 0/1、"0"/"1"/true/false 等
func asBool(v any) (bool, bool) {
	switch t := v.(type) {
	case bool:
		return t, true
	case string:
		switch strings.TrimSpace(t) {
		case "1", "true", "online", "在线":
			return true, true
		case "0", "false", "offline", "离线":
			return false, true
		}
	case float64:
		return t != 0, true
	case json.Number:
		if f, err := t.Float64(); err == nil {
			return f != 0, true
		}
	}
	return false, false
}

// healthSummary 生成健康数据文本（只输出本次推送中出现的字段）
func healthSummary(p *hwPushPayload) string {
	var parts []string
	if p.H > 0 {
		parts = append(parts, fmt.Sprintf("心率 %d 次/分", p.H))
	}
	if p.O > 0 {
		parts = append(parts, fmt.Sprintf("血氧 %d%%", p.O))
	}
	if p.W > 0 {
		parts = append(parts, fmt.Sprintf("体温 %.1f℃", p.W))
	}
	if p.X > 0 && p.Y > 0 {
		parts = append(parts, fmt.Sprintf("血压 %d/%d mmHg", p.X, p.Y))
	}
	if p.G > 0 {
		parts = append(parts, fmt.Sprintf("血糖 %.1f mmol/L", p.G))
	}
	if p.URAC > 0 {
		parts = append(parts, fmt.Sprintf("尿酸 %.1f μmol/L", p.URAC))
	}
	if p.SIGHRV > 0 {
		parts = append(parts, fmt.Sprintf("HRV %d ms", p.SIGHRV))
	}
	if p.Breathe > 0 {
		parts = append(parts, fmt.Sprintf("呼吸 %d 次/分", p.Breathe))
	}
	if p.Bli > 0 {
		parts = append(parts, fmt.Sprintf("血脂 %.1f mmol/L", p.Bli))
	}
	return strings.Join(parts, "、")
}

// healthMap 组装设备最近健康数据（用于设备详情展示）
func healthMap(p *hwPushPayload) map[string]interface{} {
	m := map[string]interface{}{}
	if p.H > 0 {
		m["heartRate"] = p.H
	}
	if p.O > 0 {
		m["bloodOxygen"] = p.O
	}
	if p.W > 0 {
		m["temperature"] = p.W
	}
	if p.X > 0 {
		m["bpHigh"] = p.X
	}
	if p.Y > 0 {
		m["bpLow"] = p.Y
	}
	if p.G > 0 {
		m["bloodSugar"] = p.G
	}
	if p.URAC > 0 {
		m["uricAcid"] = p.URAC
	}
	if p.SIGHRV > 0 {
		m["hrv"] = p.SIGHRV
	}
	if p.Breathe > 0 {
		m["breathe"] = p.Breathe
	}
	if p.Bli > 0 {
		m["bloodLipid"] = p.Bli
	}
	return m
}

// locTextOf 拼接推送中的省市区街道
func locTextOf(p *hwPushPayload) string {
	return strings.TrimSpace(strings.Join([]string{p.Pro, p.City, p.Dist, p.Str}, ""))
}

// sosContent 组装 SOS 推送文案
func sosContent(p *hwPushPayload, d *store.Device) string {
	s := "设备「" + d.Name + "」发出SOS紧急求助！"
	if loc := locTextOf(p); loc != "" {
		s += "\n位置：" + loc
	}
	if p.Guarder != "" {
		s += "\n监护号码：" + p.Guarder
	}
	return s
}

// truncateRunes 按字符截断（微信订阅消息 thing 字段限 20 字）
func truncateRunes(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n])
}

// ==================== 平台推送接收（webhook） ====================

// hwPush 接收硬件平台数据推送（平台「个人中心 → 数据上传url」指向本接口）
//
// 应答格式遵循平台约定：Ret=Succ；未绑定设备也应答成功（避免平台反复重推）。
func (a *API) hwPush(w http.ResponseWriter, r *http.Request) {
	var p hwPushPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		log.Printf("[hwpush] 推送解析失败: %v", err)
		writePlatformAck(w, http.StatusBadRequest, "Fail", "参数解析失败")
		return
	}
	log.Printf("[hwpush] 收到平台推送 pushType=%d mid=%s uid=%s action=%d", p.PushType, p.MID, p.UID, p.Action)
	a.processHwPush(&p)
	writePlatformAck(w, http.StatusOK, "Succ", "ok")
}

// processHwPush 平台推送路由：UID/MID → 设备 → 用户 → 消息 + 微信推送
// 返回处理结果（simulateHwPush 联调时直接返回给调用方）。
func (a *API) processHwPush(p *hwPushPayload) map[string]interface{} {
	res := map[string]interface{}{
		"pushType": p.PushType,
		"mid":      p.MID,
		"uid":      p.UID,
		"time":     p.Time,
	}

	// 1. 按平台设备唯一标识找到绑定的设备（UID 优先，IMEI 兜底）
	d := a.store.GetDeviceByUID(p.UID)
	if d == nil && p.MID != "" {
		d = a.store.GetDeviceByMID(p.MID)
	}
	if d == nil {
		log.Printf("[hwpush] ⚠️ 收到未绑定设备的推送 mid=%s uid=%s pushType=%d action=%d（该设备可能尚未在小程序绑定）",
			p.MID, p.UID, p.PushType, p.Action)
		res["bound"] = false
		return res
	}
	res["bound"] = true
	res["deviceId"] = d.ID
	res["deviceName"] = d.Name

	// 2. 更新设备遥测（电量/定位/在线状态）
	loc := locTextOf(p)
	if ol, ok := asBool(p.OL); ok && !ol {
		a.store.UpdateDeviceStatus(d.ID, "offline")
	} else {
		a.store.UpdateDeviceLocation(d.ID, p.B, p.Lon, p.Lat, loc)
	}

	// 3. 按 pushType 路由到站内消息 + 微信订阅消息推送
	user := a.store.UserByID(d.UserID)
	var msg *store.Message
	var push map[string]string

	switch p.PushType {
	case 1: // SOS 求助
		a.store.UpdateDeviceStatus(d.ID, "alarm")
		msg = a.store.AddMessage(d.ID, "alarm", "🚨 紧急求助", sosContent(p, d))
		push = a.pushDeviceEvent(user, "SOS紧急求助", "设备「"+d.Name+"」发出紧急求助，请立即确认安全！")

	case 2: // 健康数据（心率/血氧/体温/血压/血糖等）
		a.store.UpdateDeviceHealth(d.ID, healthMap(p))
		if summary := healthSummary(p); summary != "" {
			msg = a.store.AddMessage(d.ID, "info", "健康数据上报", "「"+d.Name+"」"+summary)
		}

	case 3: // 定位（已更新设备位置，不生成消息避免刷屏）
		// 无站内消息

	case 4: // 通知（报警/状态）
		info, ok := hwActionMap[p.Action]
		if !ok {
			info = hwAlarmInfo{"设备通知", fmt.Sprintf("设备通知（Action=%d）", p.Action), false}
		}
		typ := "info"
		if info.alarm {
			typ = "alarm"
			a.store.UpdateDeviceStatus(d.ID, "alarm")
		} else if p.Action == -1 {
			// 在线/离线状态由步骤2已处理
		}
		content := info.content
		if loc != "" {
			content += "\n位置：" + loc
		}
		msg = a.store.AddMessage(d.ID, typ, info.title, content)
		if info.alarm {
			push = a.pushDeviceEvent(user, info.title, content)
		}

	case 8: // 微聊
		mt := "语音"
		if p.MsgType == 1 {
			mt = "文字"
		}
		msg = a.store.AddMessage(d.ID, "info", "微聊"+mt+"消息", p.Content)

	case 10: // 睡眠数据
		msg = a.store.AddMessage(d.ID, "info", "睡眠数据", "「"+d.Name+"」上报了睡眠数据")

	case 12: // 手表睡眠数据
		msg = a.store.AddMessage(d.ID, "info", "手表睡眠数据", "「"+d.Name+"」上报了睡眠数据")

	case 13: // 手表心率HRV数据
		msg = a.store.AddMessage(d.ID, "info", "心率HRV数据", "「"+d.Name+"」上报了HRV数据")

	case 15: // 设备数据（dType: 1睡眠雷达 2跌倒雷达 3温湿度 4 4G睡眠带）
		if p.DType == 2 {
			a.store.UpdateDeviceStatus(d.ID, "alarm")
			msg = a.store.AddMessage(d.ID, "alarm", "跌倒雷达报警", "跌倒雷达检测到异常，请尽快查看！")
			push = a.pushDeviceEvent(user, "跌倒雷达报警", "设备「"+d.Name+"」跌倒雷达检测到异常，请尽快查看！")
		} else {
			msg = a.store.AddMessage(d.ID, "info", "设备数据上报", "「"+d.Name+"」上报了设备数据（dType="+fmt.Sprint(p.DType)+"）")
		}

	default:
		msg = a.store.AddMessage(d.ID, "info", "设备消息", fmt.Sprintf("「%s」收到设备数据（pushType=%d）", d.Name, p.PushType))
	}

	res["message"] = msg
	res["push"] = push
	return res
}

// writePlatformAck 按平台约定应答推送
func writePlatformAck(w http.ResponseWriter, status int, ret, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"Ret": ret, "Msg": msg})
}

// pushDeviceEvent 微信订阅消息推送（小程序退出也能收到）
//
// 返回结果说明：mode=simulate/no_openid/no_quota/sent/error（见 device.go pushAlarmNotify）
func (a *API) pushDeviceEvent(user *store.User, title, content string) map[string]string {
	if user == nil {
		return map[string]string{"mode": "no_user", "msg": "设备未绑定用户"}
	}
	if a.wechat == nil || !a.wechat.Configured() {
		return map[string]string{"mode": "simulate", "msg": "未配置微信订阅消息凭据，已生成本地消息（模拟推送）"}
	}
	if user.OpenID == "" {
		return map[string]string{"mode": "no_openid", "msg": "未绑定微信 openid，无法推送订阅消息"}
	}
	if !a.store.ConsumeSubscribe(user.ID) {
		return map[string]string{"mode": "no_quota", "msg": "订阅次数不足，请在小程序内重新授权订阅"}
	}
	err := a.wechat.SendSubscribeMessage(user.OpenID, "pages/devices/devices", map[string]interface{}{
		"thing1": map[string]string{"value": truncateRunes(title, 20)},
		"time2":  map[string]string{"value": time.Now().Format("2006-01-02 15:04")},
	})
	if err != nil {
		return map[string]string{"mode": "error", "msg": err.Error()}
	}
	return map[string]string{"mode": "sent", "msg": "微信订阅消息已推送，请查看微信服务通知"}
}

// ==================== 设备绑定 / 管理 ====================

type bindDeviceReq struct {
	Name string `json:"name"` // 设备名称（如：爸爸的定位手表）
	Type string `json:"type"` // 设备类型（deviceTypes）
	IMEI string `json:"imei"` // 平台设备IMEI（设备背面/包装标签）
	UID  string `json:"uid"`  // 可选：手动指定平台UID（离线联调/平台无法查询时）
}

// bindDevice 绑定硬件平台设备：向平台校验IMEI并获取UID，绑定到当前用户
//
// 精确绑定保证：
//   - UID/MID 全局唯一（AddBoundDevice），一台硬件只能归属一个用户；
//   - 平台推送按 UID/MID 定位设备，消息必然路由到唯一绑定的用户。
func (a *API) bindDevice(w http.ResponseWriter, r *http.Request) {
	var req bindDeviceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.IMEI = strings.TrimSpace(req.IMEI)
	req.UID = strings.TrimSpace(req.UID)
	if req.Name == "" || req.IMEI == "" {
		fail(w, http.StatusBadRequest, "设备名称与IMEI不能为空")
		return
	}
	if tooLong(req.Name, maxDeviceName) {
		fail(w, http.StatusBadRequest, "设备名称最长 30 个字符")
		return
	}
	if tooLong(req.IMEI, maxSN) {
		fail(w, http.StatusBadRequest, "设备IMEI最长 50 个字符")
		return
	}
	if _, ok := deviceTypes[req.Type]; !ok {
		fail(w, http.StatusBadRequest, "不支持的设备类型")
		return
	}

	var platformInfo *hwplatform.DeviceInfo
	if req.UID != "" {
		// 手动指定 UID（离线联调 / 平台查询异常时兜底）
		platformInfo = &hwplatform.DeviceInfo{MID: req.IMEI, UID: req.UID, Name: req.Name}
	} else {
		info, err := a.hw.VerifyDevice(req.IMEI)
		if err != nil {
			fail(w, http.StatusBadRequest, "设备校验失败："+err.Error())
			return
		}
		platformInfo = info
	}

	d, err := a.store.AddBoundDevice(userFrom(r).ID, &store.Device{
		Name:          req.Name,
		Type:          req.Type,
		SN:            req.IMEI,
		MID:           platformInfo.MID,
		UID:           platformInfo.UID,
		Platform:      "yiyangiot",
		Guarder:       platformInfo.Guarder,
		Battery:       platformInfo.Battery,
		Status:        "online",
		NotifyEnabled: true,
		LastLon:       platformInfo.Lon,
		LastLat:       platformInfo.Lat,
		LastLocText:   platformInfo.LocationText(),
	})
	if err == store.ErrDeviceBound {
		fail(w, http.StatusConflict, "该设备已被其他用户绑定，无法重复绑定")
		return
	}
	if err != nil {
		log.Printf("[hwpush] 绑定设备失败: %v", err)
		fail(w, http.StatusInternalServerError, "设备绑定失败，请稍后重试")
		return
	}
	a.store.AddMessage(d.ID, "info", "设备已绑定", "「"+d.Name+"」绑定成功，开始守护")
	ok(w, d)
}

type updateDeviceReq struct {
	Name          string `json:"name"`
	NotifyEnabled *bool  `json:"notifyEnabled"`
}

// updateDevice 更新设备名称/通知开关
func (a *API) updateDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	d := a.store.GetDevice(id)
	if d == nil || d.UserID != userFrom(r).ID {
		fail(w, http.StatusNotFound, "设备不存在")
		return
	}
	var req updateDeviceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name != "" && tooLong(req.Name, maxDeviceName) {
		fail(w, http.StatusBadRequest, "设备名称最长 30 个字符")
		return
	}
	ok(w, a.store.UpdateDevice(id, req.Name, req.NotifyEnabled))
}

// hwStatus 平台对接状态（小程序/调试用）
func (a *API) hwStatus(w http.ResponseWriter, r *http.Request) {
	ok(w, map[string]interface{}{
		"configured": a.hw.Configured(),
		"baseUrl":    a.hw.BaseURL(),
		"appId":      a.hw.AppID(),
		"pushPath":   "/api/v1/hw/push",
	})
}

// ==================== 平台推送模拟（本地联调） ====================

type simulateHwPushReq struct {
	Scene    string `json:"scene"`    // sos / fall / lowBattery / smoke / location / health / notification
	DeviceID string `json:"deviceId"` // 目标设备
	Action   *int   `json:"action"`   // 可选：notification 场景指定 Action
}

// simulateHwPush 模拟一次平台推送，验证「推送 → 路由 → 消息 → 微信通知」整条链路
func (a *API) simulateHwPush(w http.ResponseWriter, r *http.Request) {
	var req simulateHwPushReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	d := a.store.GetDevice(req.DeviceID)
	if d == nil || d.UserID != userFrom(r).ID {
		fail(w, http.StatusNotFound, "设备不存在")
		return
	}
	p := &hwPushPayload{
		MID:  d.MID,
		UID:  d.UID,
		Name: d.Name,
		Time: time.Now().Format("2006-01-02 15:04:05"),
	}
	// 统一填充示例定位
	setLoc := func() {
		p.Lon, p.Lat = 113.3161749, 23.1246395
		p.Pro, p.City, p.Dist, p.Str = "广东省", "广州市", "天河区", "体育西路"
	}
	switch req.Scene {
	case "sos":
		p.PushType = 1
		p.Content = "向您发出求救"
		p.Guarder = "18200000000"
		setLoc()
	case "fall":
		p.PushType = 4
		p.Action = 11
		setLoc()
	case "lowBattery":
		p.PushType = 4
		p.Action = 9
		p.B = 12
		setLoc()
	case "smoke":
		p.PushType = 4
		p.Action = 124
		setLoc()
	case "location":
		p.PushType = 3
		p.B = 88
		setLoc()
	case "health":
		p.PushType = 2
		p.H = 72
		p.O = 98
		p.W = 36.5
		p.X, p.Y = 118, 76
		p.G = 5.6
	case "notification":
		p.PushType = 4
		act := 35
		if req.Action != nil {
			act = *req.Action
		}
		p.Action = act
		setLoc()
	default:
		fail(w, http.StatusBadRequest, "不支持的场景（sos/fall/lowBattery/smoke/location/health/notification）")
		return
	}
	ok(w, a.processHwPush(p))
}
