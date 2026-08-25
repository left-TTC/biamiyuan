// commission.go
// 佣金结算（服务端持久化）：
//   - 邀请关系绑定：被邀请人绑定邀请人的推广码，写入 users.invited_by
//   - 佣金延迟到账：订单支付后生成 pending 佣金，无理由退货期（settleDays 天）满后自动转 settled 入余额
//   - 退款防护：无理由退货期内退款 → 关联 pending 佣金取消，杜绝「下单即得佣金、退款不回收」
package store

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
)

// settleDays 无理由退货期（天）＝佣金等待到账时间（默认 7 天，可用 COMMISSION_SETTLE_DAYS 覆盖）
var settleDays = 7

// SetCommissionSettleDays 设置无理由退货期/佣金到账等待天数（main 从环境变量读取）
func SetCommissionSettleDays(days int) {
	if days <= 0 {
		days = 7
	}
	settleDays = days
}

// CommissionSettleDays 读取当前无理由退货期（天）
func CommissionSettleDays() int { return settleDays }

// Commission 佣金记录
type Commission struct {
	ID        string  `json:"id"`
	UserID    string  `json:"userId"`  // 获得佣金的用户（邀请人）
	OrderID   string  `json:"orderId"` // 关联订单（模拟佣金为空）
	Amount    float64 `json:"amount"`
	Status    string  `json:"status"` // pending 待结算 / settled 已到账 / cancelled 已取消（退款）
	PaidAt    int64   `json:"paidAt"`
	SettleAt  int64   `json:"settleAt"`  // 到账时间 = 支付时间 + 无理由退货期
	SettledAt int64   `json:"settledAt,omitempty"`
	CreatedAt int64   `json:"createdAt"`
}

// Invitee 被邀请人视图（邀请人端展示）
type Invitee struct {
	UserID            string  `json:"userId"`
	Phone             string  `json:"phone"`
	NickName          string  `json:"nickName"`
	JoinTime          int64   `json:"joinTime"`
	TotalSpend        float64 `json:"totalSpend"`        // 被邀请人累计消费（已支付订单）
	Commission        float64 `json:"commission"`        // 为邀请人带来的佣金总额（待结算+已到账）
	PendingCommission float64 `json:"pendingCommission"` // 待结算金额（未到账）
}

// GetUserByPromoterCode 按邀请码查询用户（未找到返回 nil）
func (s *Store) GetUserByPromoterCode(code string) *User {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.users {
		if strings.EqualFold(u.PromoterCode, code) {
			return u
		}
	}
	return nil
}

// BindInviter 被邀请人绑定邀请人（幂等：仅首次生效；绑定关系写入服务器）
func (s *Store) BindInviter(userID, inviterCode string) (*User, error) {
	inviter := s.GetUserByPromoterCode(inviterCode)
	if inviter == nil {
		return nil, fmt.Errorf("邀请码不存在，请确认后重试")
	}
	s.mu.Lock()
	u := s.users[userID]
	if u == nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("用户不存在")
	}
	if inviter.ID == u.ID {
		s.mu.Unlock()
		return nil, fmt.Errorf("不能绑定自己的邀请码")
	}
	if u.InvitedBy != "" {
		cur := s.users[u.InvitedBy]
		s.mu.Unlock()
		if cur != nil {
			return nil, fmt.Errorf("您已绑定邀请人「%s」，不可重复绑定", cur.NickName)
		}
		return nil, fmt.Errorf("您已绑定邀请人，不可重复绑定")
	}
	u.InvitedBy = inviter.ID
	s.mu.Unlock()
	// 持久化邀请关系（含邀请时间）
	now := time.Now().UnixMilli()
	if db := s.getDB(); db != nil {
		_, _ = db.Exec(`UPDATE users SET invited_by = ?, invited_at = ? WHERE id = ?`, inviter.ID, now, userID)
	}
	u.InvitedByName = inviter.NickName
	u.InviterCode = inviter.PromoterCode
	return u, nil
}

// CreateOrderCommission 订单支付成功后：若下单用户绑定了邀请人，生成待结算佣金。
// 金额 = 订单实付 × 10%；到账时间 = 支付时间 + 无理由退货期（settleDays 天）。
func (s *Store) CreateOrderCommission(order *Order) error {
	if order == nil {
		return nil
	}
	db := s.getDB()
	if db == nil {
		return fmt.Errorf("数据库未连接")
	}
	s.mu.RLock()
	buyer := s.users[order.UserID]
	s.mu.RUnlock()
	if buyer == nil || buyer.InvitedBy == "" {
		return nil // 未绑定邀请人，无佣金
	}
	s.mu.RLock()
	inviter := s.users[buyer.InvitedBy]
	s.mu.RUnlock()
	if inviter == nil {
		return nil
	}
	amount := round2(order.Total * 0.1)
	if amount <= 0 {
		return nil
	}
	paidAt := order.PayTime
	if paidAt <= 0 {
		paidAt = time.Now().UnixMilli()
	}
	settleAt := paidAt + int64(settleDays)*24*3600*1000
	_, err := db.Exec(
		`INSERT INTO commissions (id, user_id, order_id, amount, status, paid_at, settle_at, created_at)
         VALUES (?, ?, ?, ?, 'pending', ?, ?, ?)`,
		randID("c"), inviter.ID, order.ID, amount, paidAt, settleAt, time.Now().UnixMilli())
	return err
}

// CreateDemoCommission 模拟佣金（单机演示：模拟好友下单，无真实订单），同样延迟到账
func (s *Store) CreateDemoCommission(userID string, amount float64) (*Commission, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	s.mu.RLock()
	u := s.users[userID]
	s.mu.RUnlock()
	if u == nil {
		return nil, fmt.Errorf("用户不存在")
	}
	if amount <= 0 || amount > 100000 {
		return nil, fmt.Errorf("佣金金额无效")
	}
	now := time.Now().UnixMilli()
	settleAt := now + int64(settleDays)*24*3600*1000
	c := &Commission{
		ID:        randID("c"),
		UserID:    userID,
		Amount:    round2(amount),
		Status:    "pending",
		PaidAt:    now,
		SettleAt:  settleAt,
		CreatedAt: now,
	}
	_, err := db.Exec(
		`INSERT INTO commissions (id, user_id, order_id, amount, status, paid_at, settle_at, created_at)
         VALUES (?, ?, '', ?, 'pending', ?, ?, ?)`,
		c.ID, c.UserID, c.Amount, now, settleAt, now)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// settleMu 防止并发触发到期结算时重复入账
var settleMu sync.Mutex

// SettleDueCommissions 结算所有到期佣金：pending 且 settle_at<=now → settled 并入账到余额。
// 幂等：仅处理 pending 状态记录；懒结算，在查询佣金/余额/提现时触发。
func (s *Store) SettleDueCommissions() {
	settleMu.Lock()
	defer settleMu.Unlock()
	db := s.getDB()
	if db == nil {
		return
	}
	now := time.Now().UnixMilli()
	// 先查到期记录（游标关闭后再更新，避免单连接死锁）
	rows, err := db.Query(
		`SELECT id, user_id, amount FROM commissions WHERE status = 'pending' AND settle_at <= ?`, now)
	if err != nil {
		return
	}
	type dueItem struct {
		userID string
		amount float64
	}
	var ids []string
	var due []dueItem
	for rows.Next() {
		var id, uid string
		var amt float64
		if rows.Scan(&id, &uid, &amt) == nil {
			ids = append(ids, id)
			due = append(due, dueItem{uid, amt})
		}
	}
	rows.Close()
	if rows.Err() != nil || len(ids) == 0 {
		return
	}
	tx, err := db.Begin()
	if err != nil {
		return
	}
	for _, id := range ids {
		if _, err := tx.Exec(
			`UPDATE commissions SET status = 'settled', settled_at = ? WHERE id = ? AND status = 'pending'`,
			now, id); err != nil {
			tx.Rollback()
			return
		}
	}
	if err := tx.Commit(); err != nil {
		return
	}
	// 余额入账（事务提交后改内存并持久化）
	s.mu.Lock()
	toSync := map[string]*User{}
	for _, d := range due {
		if u := s.users[d.userID]; u != nil {
			u.Balance = round2(u.Balance + d.amount)
			u.TotalCommission = round2(u.TotalCommission + d.amount)
			toSync[d.userID] = u
		}
	}
	s.mu.Unlock()
	for _, u := range toSync {
		s.syncUser(u)
	}
}

// CancelOrderCommissions 退款时取消订单关联的待结算佣金（已结算的佣金在退货期内不会发生）
func (s *Store) CancelOrderCommissions(orderID string) {
	db := s.getDB()
	if db == nil || orderID == "" {
		return
	}
	_, _ = db.Exec(
		`UPDATE commissions SET status = 'cancelled' WHERE order_id = ? AND status = 'pending'`, orderID)
}

// ListCommissions 我的佣金记录（按时间倒序）
func (s *Store) ListCommissions(userID string) ([]Commission, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	rows, err := db.Query(
		`SELECT id, user_id, order_id, amount, status, paid_at, settle_at, settled_at, created_at
         FROM commissions WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Commission{}
	for rows.Next() {
		var c Commission
		var orderID sql.NullString
		var settledAt sql.NullInt64
		if rows.Scan(&c.ID, &c.UserID, &orderID, &c.Amount, &c.Status,
			&c.PaidAt, &c.SettleAt, &settledAt, &c.CreatedAt) != nil {
			continue
		}
		if orderID.Valid {
			c.OrderID = orderID.String
		}
		if settledAt.Valid {
			c.SettledAt = settledAt.Int64
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListInvitees 我的被邀请人（含累计消费与为我带来的佣金统计，按加入时间倒序）
func (s *Store) ListInvitees(userID string) ([]Invitee, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	rows, err := db.Query(
		`SELECT id, phone, nick_name, created_at FROM users WHERE invited_by = ? ORDER BY created_at DESC`,
		userID)
	if err != nil {
		return nil, err
	}
	type rowT struct {
		id, phone, nick string
		join            int64
	}
	var items []rowT
	for rows.Next() {
		var it rowT
		if rows.Scan(&it.id, &it.phone, &it.nick, &it.join) == nil {
			items = append(items, it)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := []Invitee{}
	for _, it := range items {
		inv := Invitee{UserID: it.id, Phone: it.phone, NickName: it.nick, JoinTime: it.join}
		// 累计消费（已支付/已发货/已完成订单）
		var spend float64
		_ = db.QueryRow(
			`SELECT COALESCE(SUM(total), 0) FROM orders
             WHERE user_id = ? AND status IN ('paid', 'shipped', 'done')`, it.id).Scan(&spend)
		inv.TotalSpend = round2(spend)
		// 为我带来的佣金（该被邀请人订单产生的 pending+settled 佣金）
		var total, pending float64
		_ = db.QueryRow(
			`SELECT COALESCE(SUM(amount), 0) FROM commissions
             WHERE user_id = ? AND status IN ('pending', 'settled')
               AND order_id IN (SELECT id FROM orders WHERE user_id = ?)`,
			userID, it.id).Scan(&total)
		_ = db.QueryRow(
			`SELECT COALESCE(SUM(amount), 0) FROM commissions
             WHERE user_id = ? AND status = 'pending'
               AND order_id IN (SELECT id FROM orders WHERE user_id = ?)`,
			userID, it.id).Scan(&pending)
		inv.Commission = round2(total)
		inv.PendingCommission = round2(pending)
		out = append(out, inv)
	}
	return out, nil
}
