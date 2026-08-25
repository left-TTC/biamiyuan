// store.go
// 存储层：内存存储 + SQLite 持久化（商品审计等）
//
// 说明：主业务数据暂存内存（演示），商品修改审计写入 SQLite；
// 接入 MySQL 等正式数据库时，将各方法替换为数据库查询即可。
package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// User 用户（统一会员身份）
type User struct {
	ID              string  `json:"id"`
	Phone           string  `json:"phone"`
	NickName        string  `json:"nickName"`
	AvatarURL       string  `json:"avatarUrl,omitempty"`
	Role            string  `json:"role"`
	Balance         float64 `json:"balance"`
	TotalCommission float64 `json:"totalCommission"`
	PromoterCode    string  `json:"promoterCode"`
	InvitedBy       string  `json:"invitedBy,omitempty"`      // 邀请人 user_id（被邀请人绑定）
	InvitedByName   string  `json:"invitedByName,omitempty"`  // 邀请人昵称（展示用）
	InviterCode     string  `json:"inviterCode,omitempty"`    // 邀请人的推广码（展示用，客户端恢复本地展示）
	NotifyEnabled   bool    `json:"notifyEnabled"`
	OpenID          string  `json:"openid,omitempty"` // 微信 openid（订阅消息推送目标）
	CreatedAt       int64   `json:"createdAt"`
}

// Device 安全监护设备
//
// 硬件开放平台对接字段：
//   - MID：平台设备IMEI（与 SN 相同）
//   - UID：平台设备唯一UID，是平台推送/指令下发的唯一标识
//   - Platform：硬件平台标识（如 "yiyangiot"）
//   - LastLon/LastLat/LastLocText：最近定位（pushType=3 / 通知推送携带位置时更新）
//   - LastHealth：最近健康数据（pushType=2，心率/血氧/体温/血压/血糖等）
type Device struct {
	ID            string                 `json:"id"`
	UserID        string                 `json:"userId"`
	Name          string                 `json:"name"`
	Type          string                 `json:"type"`
	SN            string                 `json:"sn"`                   // 序列号（兼容旧数据）
	MID           string                 `json:"mid,omitempty"`        // 平台设备IMEI
	UID           string                 `json:"uid,omitempty"`        // 平台设备唯一UID
	Platform      string                 `json:"platform,omitempty"`   // 硬件平台标识
	Guarder       string                 `json:"guarder,omitempty"`    // 平台监护号码
	Status        string                 `json:"status"`               // online / offline / alarm
	Battery       int                    `json:"battery"`
	NotifyEnabled bool                   `json:"notifyEnabled"`
	LastActive    int64                  `json:"lastActive"`
	CreateTime    int64                  `json:"createTime"`
	LastLon       float64                `json:"lastLon,omitempty"`
	LastLat       float64                `json:"lastLat,omitempty"`
	LastLocText   string                 `json:"lastLocText,omitempty"`
	LastHealth    map[string]interface{} `json:"lastHealth,omitempty"`
}

// Message 设备消息
type Message struct {
	ID       string `json:"id"`
	DeviceID string `json:"deviceId"`
	Type     string `json:"type"` // alarm / status / info
	Title    string `json:"title"`
	Content  string `json:"content"`
	Time     int64  `json:"time"`
	Read     bool   `json:"read"`
}

type codeEntry struct {
	Code      string
	ExpiresAt int64
}

// adminTokenEntry 管理端令牌（带过期时间、账号与角色）
type adminTokenEntry struct {
	expiresAt int64
	accountID string
	role      string
}

// adminTokenTTL 管理端令牌有效期（秒）
const adminTokenTTL = 12 * 60 * 60 * 6000

// Store 存储层
type Store struct {
	mu          sync.RWMutex
	users       map[string]*User
	byPhone     map[string]*User
	codes       map[string]codeEntry
	tokens      map[string]string
	adminTokens map[string]adminTokenEntry
	devices     map[string]*Device
	byUID       map[string]*Device // 平台设备UID → 设备（全局唯一，保证一台硬件只归属一个用户）
	byMID       map[string]*Device // 平台设备IMEI → 设备（全局唯一）
	messages    []*Message
	subscribe   map[string]int // userID -> 可用订阅消息次数（一次性订阅）
	db          *sql.DB        // SQLite 连接（可为 nil）
	dbPath      string
}

// New 创建存储实例
func New() *Store {
	return &Store{
		users:       make(map[string]*User),
		byPhone:     make(map[string]*User),
		codes:       make(map[string]codeEntry),
		tokens:      make(map[string]string),
		adminTokens: make(map[string]adminTokenEntry),
		devices:     make(map[string]*Device),
		byUID:       make(map[string]*Device),
		byMID:       make(map[string]*Device),
		messages:    make([]*Message, 0),
		subscribe:   make(map[string]int),
	}
}

// ==================== 工具 ====================

func randID(prefix string) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}

func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func suffix(phone string) string {
	if len(phone) >= 4 {
		return phone[len(phone)-4:]
	}
	return phone
}

func randCode() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 6)
	r := make([]byte, 6)
	_, _ = rand.Read(r)
	for i := range b {
		b[i] = chars[int(r[i])%len(chars)]
	}
	return string(b)
}

// ==================== 验证码 ====================

// SaveCode 保存验证码（5 分钟有效）
func (s *Store) SaveCode(phone, code string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.codes[phone] = codeEntry{Code: code, ExpiresAt: time.Now().Add(5 * time.Minute).Unix()}
}

// VerifyCode 校验验证码
func (s *Store) VerifyCode(phone, code string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.codes[phone]
	if !ok || e.Code != code {
		return false
	}
	return time.Now().Unix() <= e.ExpiresAt
}

// ==================== 用户与登录 ====================

// UserByPhone 通过手机号查询用户（未注册返回 nil）
func (s *Store) UserByPhone(phone string) *User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.byPhone[phone]
}

// CreateUser 创建新用户（注册），手机号已注册时返回 nil
// 注意：登录不再自动注册，账号必须显式通过注册接口创建
func (s *Store) CreateUser(phone string) *User {
	s.mu.Lock()
	if _, ok := s.byPhone[phone]; ok {
		s.mu.Unlock()
		return nil
	}
	u := &User{
		ID:           randID("u"),
		Phone:        phone,
		NickName:     "会员" + suffix(phone),
		Role:         "member",
		PromoterCode: randCode(),
		CreatedAt:    time.Now().UnixMilli(),
	}
	s.users[u.ID] = u
	s.byPhone[phone] = u
	s.mu.Unlock()
	s.syncUser(u)
	return u
}

// CreateToken 为用户创建登录令牌
func (s *Store) CreateToken(userID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := newToken()
	s.tokens[t] = userID
	return t
}

// UserByToken 通过令牌获取用户
func (s *Store) UserByToken(token string) *User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.tokens[token]
	if !ok {
		return nil
	}
	return s.users[id]
}

// UserByID 通过用户 ID 查询用户
func (s *Store) UserByID(userID string) *User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.users[userID]
}

// ==================== 用户持久化（SQLite） ====================

// LoadUsers 从 SQLite 加载全部用户到内存（服务器重启后恢复账号，
// 保证订单/地址/提现等与账号绑定数据仍可关联；登录 token 失效时客户端自动重登）
func (s *Store) LoadUsers() {
	db := s.getDB()
	if db == nil {
		return
	}
	rows, err := db.Query(
		`SELECT id, phone, nick_name, avatar_url, role, balance, total_commission,
                promoter_code, invited_by, notify_enabled, openid, created_at FROM users`)
	if err != nil {
		return
	}
	defer rows.Close()
	loaded := 0
	s.mu.Lock()
	for rows.Next() {
		var u User
		var notify int
		var invitedBy string
		if rows.Scan(&u.ID, &u.Phone, &u.NickName, &u.AvatarURL, &u.Role,
			&u.Balance, &u.TotalCommission, &u.PromoterCode, &invitedBy, &notify, &u.OpenID, &u.CreatedAt) != nil {
			continue
		}
		u.InvitedBy = invitedBy
		u.NotifyEnabled = notify == 1
		s.users[u.ID] = &u
		s.byPhone[u.Phone] = &u
		loaded++
	}
	s.mu.Unlock()
	// 补全邀请人昵称与推广码（跨用户引用，锁外读取仅内存）
	s.mu.RLock()
	for _, u := range s.users {
		if u.InvitedBy != "" {
			if inv := s.users[u.InvitedBy]; inv != nil {
				u.InvitedByName = inv.NickName
				u.InviterCode = inv.PromoterCode
			}
		}
	}
	s.mu.RUnlock()
	if loaded > 0 {
		// 日志交给调用方
	}
}

// syncUser 将用户账号持久化到 SQLite（幂等 upsert；余额/佣金变更后同步）
func (s *Store) syncUser(u *User) {
	db := s.getDB()
	if db == nil || u == nil {
		return
	}
	notify := 0
	if u.NotifyEnabled {
		notify = 1
	}
	_, _ = db.Exec(
		`INSERT INTO users (id, phone, nick_name, avatar_url, role, balance, total_commission, promoter_code, invited_by, notify_enabled, openid, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
         ON CONFLICT(id) DO UPDATE SET
            phone=excluded.phone, nick_name=excluded.nick_name, avatar_url=excluded.avatar_url,
            role=excluded.role, balance=excluded.balance, total_commission=excluded.total_commission,
            promoter_code=excluded.promoter_code, invited_by=excluded.invited_by,
            notify_enabled=excluded.notify_enabled,
            openid=excluded.openid`,
		u.ID, u.Phone, u.NickName, u.AvatarURL, u.Role, u.Balance, u.TotalCommission,
		u.PromoterCode, u.InvitedBy, notify, u.OpenID, u.CreatedAt)
}

// ==================== 微信 openid 与订阅 ====================

// SetOpenID 为用户绑定微信 openid
func (s *Store) SetOpenID(userID, openid string) {
	s.mu.Lock()
	if u, ok := s.users[userID]; ok {
		u.OpenID = openid
		s.mu.Unlock()
		s.syncUser(u)
		return
	}
	s.mu.Unlock()
}

// UpdateUserProfile 更新用户资料（昵称 / 头像），返回更新后的用户
func (s *Store) UpdateUserProfile(userID, nickName, avatarURL string) *User {
	s.mu.Lock()
	u := s.users[userID]
	if u == nil {
		s.mu.Unlock()
		return nil
	}
	if nickName != "" {
		u.NickName = nickName
	}
	if avatarURL != "" {
		u.AvatarURL = avatarURL
	}
	s.mu.Unlock()
	s.syncUser(u)
	return u
}

// AddCommission 增加用户佣金（可提现余额 + 累计佣金），并持久化。
// 被邀请人订单支付后，邀请人佣金上报到服务器，提现在服务器扣减余额。
func (s *Store) AddCommission(userID string, amount float64) *User {
	s.mu.Lock()
	u := s.users[userID]
	if u == nil {
		s.mu.Unlock()
		return nil
	}
	u.Balance = round2(u.Balance + amount)
	u.TotalCommission = round2(u.TotalCommission + amount)
	s.mu.Unlock()
	s.syncUser(u)
	return u
}

// AddSubscribeQuota 增加用户订阅消息次数（wx.requestSubscribeMessage 授权成功后调用）
func (s *Store) AddSubscribeQuota(userID string, n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscribe[userID] += n
}

// ConsumeSubscribe 消耗一次订阅消息额度；额度不足返回 false
func (s *Store) ConsumeSubscribe(userID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.subscribe[userID] <= 0 {
		return false
	}
	s.subscribe[userID]--
	return true
}

// ==================== 设备 ====================

// ErrDeviceBound 设备已被其他用户绑定
var ErrDeviceBound = errors.New("设备已被绑定")

// AddDevice 添加设备（本地/演示用，不校验平台UID/MID唯一性）
func (s *Store) AddDevice(userID string, d *Device) *Device {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	d.ID = randID("d")
	d.UserID = userID
	d.Status = "online"
	d.Battery = 55 + int(now.Unix()%40)
	d.LastActive = now.UnixMilli()
	d.CreateTime = now.UnixMilli()
	s.devices[d.ID] = d
	return d
}

// AddBoundDevice 绑定硬件平台设备：UID/MID 全局唯一（一台硬件只归属一个用户）
//
// 任一 UID/MID 已存在时返回 ErrDeviceBound，保证「精确绑定」。
func (s *Store) AddBoundDevice(userID string, d *Device) (*Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d.UID != "" {
		if existing, ok := s.byUID[d.UID]; ok {
			return existing, ErrDeviceBound
		}
	}
	if d.MID != "" {
		if existing, ok := s.byMID[d.MID]; ok {
			return existing, ErrDeviceBound
		}
	}
	now := time.Now()
	d.ID = randID("d")
	d.UserID = userID
	if d.Status == "" {
		d.Status = "online"
	}
	if d.Battery <= 0 {
		d.Battery = 55 + int(now.Unix()%40)
	}
	d.LastActive = now.UnixMilli()
	d.CreateTime = now.UnixMilli()
	s.devices[d.ID] = d
	if d.UID != "" {
		s.byUID[d.UID] = d
	}
	if d.MID != "" {
		s.byMID[d.MID] = d
	}
	return d, nil
}

// GetDevice 获取设备
func (s *Store) GetDevice(id string) *Device {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.devices[id]
}

// GetDeviceByUID 按平台设备UID查找设备（硬件推送路由：UID → 设备 → 用户）
func (s *Store) GetDeviceByUID(uid string) *Device {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if uid == "" {
		return nil
	}
	return s.byUID[uid]
}

// GetDeviceByMID 按平台设备IMEI查找设备（UID缺失时的兜底路由）
func (s *Store) GetDeviceByMID(mid string) *Device {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if mid == "" {
		return nil
	}
	return s.byMID[mid]
}

// ListDevices 获取用户的设备列表
func (s *Store) ListDevices(userID string) []*Device {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Device, 0)
	for _, d := range s.devices {
		if d.UserID == userID {
			out = append(out, d)
		}
	}
	return out
}

// RemoveDevice 移除设备（同时删除其消息）
func (s *Store) RemoveDevice(userID, id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.devices[id]
	if !ok || d.UserID != userID {
		return false
	}
	delete(s.devices, id)
	if d.UID != "" {
		delete(s.byUID, d.UID)
	}
	if d.MID != "" {
		delete(s.byMID, d.MID)
	}
	msgs := s.messages[:0]
	for _, m := range s.messages {
		if m.DeviceID != id {
			msgs = append(msgs, m)
		}
	}
	s.messages = msgs
	return true
}

// UpdateDeviceStatus 更新设备状态与最后活跃时间
func (s *Store) UpdateDeviceStatus(id, status string) *Device {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.devices[id]
	if d == nil {
		return nil
	}
	d.Status = status
	d.LastActive = time.Now().UnixMilli()
	return d
}

// UpdateDeviceLocation 更新设备定位/电量并置为在线（硬件平台定位/通知推送时调用）
func (s *Store) UpdateDeviceLocation(id string, battery int, lon, lat float64, locText string) *Device {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.devices[id]
	if d == nil {
		return nil
	}
	d.Status = "online"
	if battery > 0 {
		d.Battery = battery
	}
	if lon != 0 || lat != 0 {
		d.LastLon = lon
		d.LastLat = lat
	}
	if locText != "" {
		d.LastLocText = locText
	}
	d.LastActive = time.Now().UnixMilli()
	return d
}

// UpdateDeviceHealth 更新设备最近健康数据（心率/血氧/体温/血压/血糖等）
func (s *Store) UpdateDeviceHealth(id string, health map[string]interface{}) *Device {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.devices[id]
	if d == nil {
		return nil
	}
	if len(health) > 0 {
		d.LastHealth = health
	}
	d.LastActive = time.Now().UnixMilli()
	return d
}

// UpdateDevice 通用更新（设备名称 / 通知开关）
func (s *Store) UpdateDevice(id, name string, notify *bool) *Device {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.devices[id]
	if d == nil {
		return nil
	}
	if name != "" {
		d.Name = name
	}
	if notify != nil {
		d.NotifyEnabled = *notify
	}
	return d
}

// ==================== 消息 ====================

// AddMessage 新增设备消息（置顶插入）
func (s *Store) AddMessage(deviceID, typ, title, content string) *Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := &Message{
		ID:       randID("m"),
		DeviceID: deviceID,
		Type:     typ,
		Title:    title,
		Content:  content,
		Time:     time.Now().UnixMilli(),
		Read:     false,
	}
	s.messages = append([]*Message{m}, s.messages...)
	return m
}

// ListMessages 获取用户所有设备的消息（按时间倒序）
func (s *Store) ListMessages(userID string) []*Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	owned := make(map[string]bool)
	for _, d := range s.devices {
		if d.UserID == userID {
			owned[d.ID] = true
		}
	}
	out := make([]*Message, 0)
	for _, m := range s.messages {
		if owned[m.DeviceID] {
			out = append(out, m)
		}
	}
	return out
}

// MarkRead 标记消息已读
func (s *Store) MarkRead(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.messages {
		if m.ID == id {
			m.Read = true
			return true
		}
	}
	return false
}

// ==================== 管理端 ====================

// CreateAdminToken 创建管理端令牌（绑定账号与角色，有效期 12 小时）
func (s *Store) CreateAdminToken(accountID, role string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := newToken()
	s.adminTokens[t] = adminTokenEntry{
		expiresAt: time.Now().Unix() + adminTokenTTL,
		accountID: accountID,
		role:      role,
	}
	return t
}

// VerifyAdminToken 校验管理端令牌（含过期校验），返回 (账号ID, 角色, 是否有效)
func (s *Store) VerifyAdminToken(token string) (string, string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.adminTokens[token]
	if !ok {
		return "", "", false
	}
	if time.Now().Unix() > e.expiresAt {
		return "", "", false
	}
	return e.accountID, e.role, true
}

// Stats 管理端统计数据
func (s *Store) Stats() map[string]int {
	s.mu.RLock()
	users := len(s.users)
	devices := len(s.devices)
	messages := len(s.messages)
	alarms := 0
	for _, d := range s.devices {
		if d.Status == "alarm" {
			alarms++
		}
	}
	s.mu.RUnlock()

	return map[string]int{
		"users":      users,
		"devices":    devices,
		"messages":   messages,
		"products":   s.CountProducts(),
		"categories": s.CountCategories(),
		"alarms":     alarms,
	}
}

// ListUsers 全部用户
func (s *Store) ListUsers() []*User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, u)
	}
	return out
}

// ListAllDevices 全部设备
func (s *Store) ListAllDevices() []*Device {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Device, 0, len(s.devices))
	for _, d := range s.devices {
		out = append(out, d)
	}
	return out
}

// ListAllMessages 全部消息
func (s *Store) ListAllMessages() []*Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Message, len(s.messages))
	copy(out, s.messages)
	return out
}
