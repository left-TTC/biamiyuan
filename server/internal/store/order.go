// order.go
// 订单、收货地址、提现的存储（SQLite 持久化，绑定用户账号）
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ==================== 收货地址 ====================

// Address 收货地址（绑定用户账号）
type Address struct {
	ID        string `json:"id"`
	UserID    string `json:"userId"`
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	Region    string `json:"region"`
	Detail    string `json:"detail"`
	IsDefault bool   `json:"isDefault"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// ListAddresses 获取用户的收货地址列表（默认地址优先，其余按更新时间倒序）
func (s *Store) ListAddresses(userID string) ([]Address, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	rows, err := db.Query(
		`SELECT id, user_id, name, phone, region, detail, is_default, created_at, updated_at
         FROM addresses WHERE user_id = ?
         ORDER BY is_default DESC, updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Address{}
	for rows.Next() {
		var a Address
		var def int
		if rows.Scan(&a.ID, &a.UserID, &a.Name, &a.Phone, &a.Region, &a.Detail, &def, &a.CreatedAt, &a.UpdatedAt) != nil {
			continue
		}
		a.IsDefault = def == 1
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetAddress 查询地址（须属于该用户）
func (s *Store) GetAddress(userID, id string) (*Address, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	var a Address
	var def int
	err := db.QueryRow(
		`SELECT id, user_id, name, phone, region, detail, is_default, created_at, updated_at
         FROM addresses WHERE id = ? AND user_id = ?`, id, userID).
		Scan(&a.ID, &a.UserID, &a.Name, &a.Phone, &a.Region, &a.Detail, &def, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.IsDefault = def == 1
	return &a, nil
}

// SaveAddress 新增或更新地址（isDefault=true 时自动把该用户其他地址默认取消）
func (s *Store) SaveAddress(userID string, a Address) (*Address, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	now := time.Now().UnixMilli()
	// 更新前先查地址归属（须在事务外，避免占用唯一连接导致死锁）
	var existing *Address
	if a.ID != "" {
		var err error
		existing, err = s.GetAddress(userID, a.ID)
		if err != nil {
			return nil, err
		}
		if existing == nil {
			return nil, fmt.Errorf("地址不存在")
		}
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	if a.ID == "" {
		a.ID = randID("a")
		a.UserID = userID
		a.CreatedAt = now
		a.UpdatedAt = now
		if _, err := tx.Exec(
			`INSERT INTO addresses (id, user_id, name, phone, region, detail, is_default, created_at, updated_at)
             VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			a.ID, userID, a.Name, a.Phone, a.Region, a.Detail, boolInt(a.IsDefault), a.CreatedAt, a.UpdatedAt); err != nil {
			tx.Rollback()
			return nil, err
		}
	} else {
		// 更新：保留原创建时间
		a.UserID = userID
		a.CreatedAt = existing.CreatedAt
		a.UpdatedAt = now
		if _, err := tx.Exec(
			`UPDATE addresses SET name=?, phone=?, region=?, detail=?, is_default=?, updated_at=? WHERE id=?`,
			a.Name, a.Phone, a.Region, a.Detail, boolInt(a.IsDefault), a.UpdatedAt, a.ID); err != nil {
			tx.Rollback()
			return nil, err
		}
	}
	if a.IsDefault {
		if _, err := tx.Exec(`UPDATE addresses SET is_default = 0 WHERE user_id = ? AND id != ?`, userID, a.ID); err != nil {
			tx.Rollback()
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &a, nil
}

// DeleteAddress 删除地址（删除后若没有默认地址，则把第一条设为默认）
func (s *Store) DeleteAddress(userID, id string) error {
	db := s.getDB()
	if db == nil {
		return fmt.Errorf("数据库未连接")
	}
	if _, err := db.Exec(`DELETE FROM addresses WHERE id = ? AND user_id = ?`, id, userID); err != nil {
		return err
	}
	var cnt int
	_ = db.QueryRow(`SELECT COUNT(*) FROM addresses WHERE user_id = ? AND is_default = 1`, userID).Scan(&cnt)
	if cnt == 0 {
		var firstID string
		err := db.QueryRow(`SELECT id FROM addresses WHERE user_id = ? ORDER BY updated_at DESC LIMIT 1`, userID).Scan(&firstID)
		if err == nil {
			_, _ = db.Exec(`UPDATE addresses SET is_default = 1 WHERE id = ?`, firstID)
		}
	}
	return nil
}

// SetDefaultAddress 将指定地址设为默认（其余取消）
func (s *Store) SetDefaultAddress(userID, id string) error {
	db := s.getDB()
	if db == nil {
		return fmt.Errorf("数据库未连接")
	}
	existing, err := s.GetAddress(userID, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("地址不存在")
	}
	now := time.Now().UnixMilli()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE addresses SET is_default = 0 WHERE user_id = ?`, userID); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.Exec(`UPDATE addresses SET is_default = 1, updated_at = ? WHERE id = ?`, now, id); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// ==================== 订单 ====================

// AttrSelection 已选属性
type AttrSelection struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// OrderItemInput 下单条目
type OrderItemInput struct {
	ProductID string          `json:"productId"`
	Quantity  int             `json:"quantity"`
	Attrs     []AttrSelection `json:"attrs"`
}

// OrderItem 订单商品快照
type OrderItem struct {
	ID         string          `json:"id"`
	ProductID  string          `json:"productId"`
	Name       string          `json:"name"`
	Price      float64         `json:"price"`
	Count      int             `json:"count"`
	Emoji      string          `json:"emoji"`
	Colors     []string        `json:"colors"`
	Images     []string        `json:"images"`
	Attrs      []AttrSelection `json:"attrs"`
	AttrsText  string          `json:"attrsText"`
	Service    bool            `json:"service"`
	SourceTeam string          `json:"sourceTeam"`
}

// Order 订单（含支付与物流信息）
type Order struct {
	ID          string      `json:"id"`
	OrderNo     string      `json:"orderNo"`
	UserID      string      `json:"userId"`
	Status      string      `json:"status"` // pending / paid / shipped / done / canceled / refunded
	Total       float64     `json:"total"`
	Address     interface{} `json:"address"` // 下单时地址快照（对象）
	Remark      string      `json:"remark"`
	PayMethod   string      `json:"payMethod"`
	PayTime     int64       `json:"payTime"`
	TransactionID string    `json:"transactionId,omitempty"` // 微信支付交易号（回调写入）
	ShipCompany string      `json:"shipCompany"`
	ShipNo      string      `json:"shipNo"`
	ShipTime    int64       `json:"shipTime"`
	FinishTime  int64       `json:"finishTime"`
	CancelTime  int64       `json:"cancelTime"`
	RefundTime  int64       `json:"refundTime,omitempty"`
	CreateTime  int64       `json:"createTime"`
	Items       []OrderItem `json:"items"`
}

// CreateOrder 创建订单：服务器根据商品价格计算总额，保存商品快照与地址快照
func (s *Store) CreateOrder(userID, addressID, remark string, inputs []OrderItemInput) (*Order, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	if len(inputs) == 0 {
		return nil, fmt.Errorf("订单商品为空")
	}
	addr, err := s.GetAddress(userID, addressID)
	if err != nil {
		return nil, err
	}
	if addr == nil {
		return nil, fmt.Errorf("收货地址不存在，请先添加地址")
	}
	addressJSON, _ := json.Marshal(addr)

	// 组装商品快照（价格以服务器商品表为准）
	items := []OrderItem{}
	var total float64
	for _, in := range inputs {
		if in.Quantity <= 0 {
			in.Quantity = 1
		}
		p, err := s.GetProduct(in.ProductID)
		if err != nil {
			return nil, err
		}
		if p == nil {
			return nil, fmt.Errorf("商品不存在: %s", in.ProductID)
		}
		attrsText := ""
		if len(in.Attrs) > 0 {
			parts := []string{}
			for _, at := range in.Attrs {
				parts = append(parts, at.Name+":"+at.Value)
			}
			attrsText = strings.Join(parts, " / ")
		}
		colors := p.Colors
		if len(colors) == 0 {
			colors = []string{"#3B82F6", "#1E40AF"}
		}
		items = append(items, OrderItem{
			ProductID:  p.ID,
			Name:       p.Name,
			Price:      p.Price,
			Count:      in.Quantity,
			Emoji:      p.Emoji,
			Colors:     colors,
			Images:     p.Images,
			Attrs:      in.Attrs,
			AttrsText:  attrsText,
			Service:    p.Service,
			SourceTeam: p.SourceTeam,
		})
		total += p.Price * float64(in.Quantity)
	}

	o := &Order{
		ID:         randID("o"),
		OrderNo:    genOrderNo(),
		UserID:     userID,
		Status:     "pending",
		Total:      total,
		Remark:     remark,
		CreateTime: time.Now().UnixMilli(),
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(
		`INSERT INTO orders (id, order_no, user_id, status, total_amount, address_json, remark, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		o.ID, o.OrderNo, userID, o.Status, o.Total, addressJSON, o.Remark, o.CreateTime, o.CreateTime); err != nil {
		tx.Rollback()
		return nil, err
	}
	for _, it := range items {
		itemID := randID("oi")
		imagesJSON, _ := json.Marshal(it.Images)
		attrsJSON, _ := json.Marshal(it.Attrs)
		if _, err := tx.Exec(
			`INSERT INTO order_items (id, order_id, product_id, name, price, quantity, emoji, images, attrs, service, source_team, created_at)
             VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			itemID, o.ID, it.ProductID, it.Name, it.Price, it.Count, it.Emoji, imagesJSON, attrsJSON, boolInt(it.Service), it.SourceTeam, o.CreateTime); err != nil {
			tx.Rollback()
			return nil, err
		}
		it.ID = itemID
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	o.Items = items
	o.Address = addr
	return o, nil
}


// GetOrder 获取订单详情（含商品明细）
func (s *Store) GetOrder(userID, id string) (*Order, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	var o Order
	var addressJSON string
	var shipCompany, shipNo sql.NullString
	var payTime, shipTime, finishTime, cancelTime, refundTime sql.NullInt64
	err := db.QueryRow(
		`SELECT id, order_no, user_id, status, total_amount, address_json, remark, pay_method, pay_time, transaction_id, ship_company, ship_no, ship_time, finish_time, cancel_time, refund_time, created_at
         FROM orders WHERE id = ? AND user_id = ?`, id, userID).
		Scan(&o.ID, &o.OrderNo, &o.UserID, &o.Status, &o.Total, &addressJSON, &o.Remark, &o.PayMethod,
			&payTime, &o.TransactionID, &shipCompany, &shipNo, &shipTime, &finishTime, &cancelTime, &refundTime, &o.CreateTime)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	o.PayTime = nullInt64(payTime)
	o.ShipTime = nullInt64(shipTime)
	o.FinishTime = nullInt64(finishTime)
	o.CancelTime = nullInt64(cancelTime)
	o.RefundTime = nullInt64(refundTime)
	o.ShipCompany = nullString(shipCompany)
	o.ShipNo = nullString(shipNo)
	var addr map[string]interface{}
	_ = json.Unmarshal([]byte(addressJSON), &addr)
	o.Address = addr
	items, err := s.scanOrderItems(db, o.ID)
	if err != nil {
		return nil, err
	}
	o.Items = items
	return &o, nil
}

// ListOrders 订单列表（status=all 或留空返回全部）
func (s *Store) ListOrders(userID, status string) ([]Order, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	where := "WHERE user_id = ?"
	args := []interface{}{userID}
	if status != "" && status != "all" {
		where += " AND status = ?"
		args = append(args, status)
	}
	rows, err := db.Query(`SELECT id, order_no, user_id, status, total_amount, address_json, remark, pay_method, pay_time, transaction_id, ship_company, ship_no, ship_time, finish_time, cancel_time, refund_time, created_at FROM orders `+where+` ORDER BY created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	out := []Order{}
	for rows.Next() {
		var o Order
		var addressJSON string
		var shipCompany, shipNo sql.NullString
		var payTime, shipTime, finishTime, cancelTime, refundTime sql.NullInt64
		if rows.Scan(&o.ID, &o.OrderNo, &o.UserID, &o.Status, &o.Total, &addressJSON, &o.Remark, &o.PayMethod,
			&payTime, &o.TransactionID, &shipCompany, &shipNo, &shipTime, &finishTime, &cancelTime, &refundTime, &o.CreateTime) != nil {
			continue
		}
		o.PayTime = nullInt64(payTime)
		o.ShipTime = nullInt64(shipTime)
		o.FinishTime = nullInt64(finishTime)
		o.CancelTime = nullInt64(cancelTime)
	o.RefundTime = nullInt64(refundTime)
		o.ShipCompany = nullString(shipCompany)
		o.ShipNo = nullString(shipNo)
		var addr map[string]interface{}
		_ = json.Unmarshal([]byte(addressJSON), &addr)
		o.Address = addr
		out = append(out, o)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// 游标关闭后再逐单查商品明细，避免 SQLite 单连接死锁
	for i := range out {
		items, err := s.scanOrderItems(db, out[i].ID)
		if err != nil {
			continue
		}
		out[i].Items = items
	}
	return out, nil
}

func (s *Store) scanOrderItems(db *sql.DB, orderID string) ([]OrderItem, error) {
	rows, err := db.Query(
		`SELECT id, product_id, name, price, quantity, emoji, images, attrs, service, source_team
         FROM order_items WHERE order_id = ? ORDER BY created_at`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OrderItem{}
	for rows.Next() {
		var it OrderItem
		var imagesJSON, attrsJSON string
		var service int
		if rows.Scan(&it.ID, &it.ProductID, &it.Name, &it.Price, &it.Count, &it.Emoji, &imagesJSON, &attrsJSON, &service, &it.SourceTeam) != nil {
			continue
		}
		it.Service = service == 1
		_ = json.Unmarshal([]byte(imagesJSON), &it.Images)
		_ = json.Unmarshal([]byte(attrsJSON), &it.Attrs)
		if len(it.Attrs) > 0 {
			parts := []string{}
			for _, at := range it.Attrs {
				parts = append(parts, at.Name+":"+at.Value)
			}
			it.AttrsText = strings.Join(parts, " / ")
		}
		if len(it.Colors) == 0 {
			it.Colors = []string{"#3B82F6", "#1E40AF"}
		}
		out = append(out, it)
	}
	return out, rows.Err()
}


// UpdateOrderStatus 更新订单状态（paid 记录支付时间、done 记录完成时间、canceled 记录取消时间）
func (s *Store) UpdateOrderStatus(userID, id, status string) (*Order, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	o, err := s.GetOrder(userID, id)
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, fmt.Errorf("订单不存在")
	}
	now := time.Now().UnixMilli()
	var col string
	switch status {
	case "paid":
		col = "pay_time"
	case "done":
		col = "finish_time"
	case "canceled":
		col = "cancel_time"
	default:
		col = "updated_at"
	}
	if _, err := db.Exec(
		fmt.Sprintf(`UPDATE orders SET status = ?, %s = ?, updated_at = ? WHERE id = ?`, col),
		status, now, now, id); err != nil {
		return nil, err
	}
	return s.GetOrder(userID, id)
}

// UpdateOrderShip 后台发货：绑定物流公司与物流单号
func (s *Store) UpdateOrderShip(id, company, shipNo string) (*Order, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	var uid string
	err := db.QueryRow(`SELECT user_id FROM orders WHERE id = ?`, id).Scan(&uid)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("订单不存在")
	}
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	if _, err := db.Exec(
		`UPDATE orders SET status = 'shipped', ship_company = ?, ship_no = ?, ship_time = ?, updated_at = ? WHERE id = ?`,
		company, shipNo, now, now, id); err != nil {
		return nil, err
	}
	return s.GetOrder(uid, id)
}

// ListAllOrders 全部订单（管理端）
func (s *Store) ListAllOrders(status string) ([]Order, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	where := ""
	args := []interface{}{}
	if status != "" && status != "all" {
		where = "WHERE status = ?"
		args = append(args, status)
	}
	rows, err := db.Query(`SELECT id, order_no, user_id, status, total_amount, address_json, remark, pay_method, pay_time, transaction_id, ship_company, ship_no, ship_time, finish_time, cancel_time, refund_time, created_at FROM orders `+where+` ORDER BY created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	out := []Order{}
	for rows.Next() {
		var o Order
		var addressJSON string
		var shipCompany, shipNo sql.NullString
		var payTime, shipTime, finishTime, cancelTime, refundTime sql.NullInt64
		if rows.Scan(&o.ID, &o.OrderNo, &o.UserID, &o.Status, &o.Total, &addressJSON, &o.Remark, &o.PayMethod,
			&payTime, &o.TransactionID, &shipCompany, &shipNo, &shipTime, &finishTime, &cancelTime, &refundTime, &o.CreateTime) != nil {
			continue
		}
		o.PayTime = nullInt64(payTime)
		o.ShipTime = nullInt64(shipTime)
		o.FinishTime = nullInt64(finishTime)
		o.CancelTime = nullInt64(cancelTime)
	o.RefundTime = nullInt64(refundTime)
		o.ShipCompany = nullString(shipCompany)
		o.ShipNo = nullString(shipNo)
		var addr map[string]interface{}
		_ = json.Unmarshal([]byte(addressJSON), &addr)
		o.Address = addr
		out = append(out, o)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// 游标关闭后再逐单查商品明细，避免 SQLite 单连接死锁
	for i := range out {
		items, err := s.scanOrderItems(db, out[i].ID)
		if err != nil {
			continue
		}
		out[i].Items = items
	}
	return out, nil
}

// MarkOrderPaid 标记订单已支付（记录支付方式与微信交易号；模拟模式 transactionID 传空）
func (s *Store) MarkOrderPaid(userID, id, payMethod, transactionID string) (*Order, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	o, err := s.GetOrder(userID, id)
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, fmt.Errorf("订单不存在")
	}
	if o.Status == "paid" || o.Status == "shipped" || o.Status == "done" {
		return o, nil
	}
	if o.Status == "canceled" {
		return nil, fmt.Errorf("订单已取消，无法支付")
	}
	now := time.Now().UnixMilli()
	if _, err := db.Exec(
		`UPDATE orders SET status = 'paid', pay_method = ?, pay_time = ?, transaction_id = ?, updated_at = ? WHERE id = ?`,
		payMethod, now, transactionID, now, id); err != nil {
		return nil, err
	}
	return s.GetOrder(userID, id)
}

// MarkOrderPaidByOrderNo 按商户订单号（order_no）标记已支付。
// 微信支付回调调用（无用户上下文）；重复回调幂等返回当前订单。
func (s *Store) MarkOrderPaidByOrderNo(orderNo, payMethod, transactionID string) (*Order, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	var id, userID string
	if err := db.QueryRow(`SELECT id, user_id FROM orders WHERE order_no = ?`, orderNo).Scan(&id, &userID); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("订单不存在")
		}
		return nil, err
	}
	o, err := s.GetOrder(userID, id)
	if err != nil {
		return nil, err
	}
	if o.Status == "paid" || o.Status == "shipped" || o.Status == "done" {
		return o, nil
	}
	if o.Status == "canceled" {
		return nil, fmt.Errorf("订单已取消，无法支付")
	}
	now := time.Now().UnixMilli()
	if _, err := db.Exec(
		`UPDATE orders SET status = 'paid', pay_method = ?, pay_time = ?, transaction_id = ?, updated_at = ? WHERE id = ?`,
		payMethod, now, transactionID, now, id); err != nil {
		return nil, err
	}
	return s.GetOrder(userID, id)
}


// RefundOrder 无理由退款（支付后无理由退货期内可退）：paid → refunded。
// 超过无理由退货期（settleDays 天）不可退款；退款后关联的待结算佣金由调用方取消。
func (s *Store) RefundOrder(userID, id string) (*Order, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	o, err := s.GetOrder(userID, id)
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, fmt.Errorf("订单不存在")
	}
	if o.Status != "paid" {
		return nil, fmt.Errorf("当前订单状态不可退款（仅已支付订单可申请无理由退款）")
	}
	if time.Now().UnixMilli() > o.PayTime+int64(settleDays)*24*3600*1000 {
		return nil, fmt.Errorf("已超过无理由退货期（%d 天），无法申请退款，请联系客服", settleDays)
	}
	now := time.Now().UnixMilli()
	if _, err := db.Exec(
		`UPDATE orders SET status = 'refunded', refund_time = ?, updated_at = ? WHERE id = ?`,
		now, now, id); err != nil {
		return nil, err
	}
	return s.GetOrder(userID, id)
}


// ==================== 提现 ====================

// Withdrawal 提现申请
type Withdrawal struct {
	ID         string  `json:"id"`
	UserID     string  `json:"userId"`
	Amount     float64 `json:"amount"`
	Fee        float64 `json:"fee"`
	Method     string  `json:"method"`
	Account    string  `json:"account"`
	BankCardNo string  `json:"bankCardNo"` // 企业银行卡号（服务器写入）
	Status     string  `json:"status"`     // processing / done / failed
	ApplyTime  int64   `json:"applyTime"`
	FinishTime int64   `json:"finishTime"`
	Remark     string  `json:"remark"`
}

// ApplyWithdrawal 申请提现（余额在事务中扣减）
func (s *Store) ApplyWithdrawal(userID, method, account string, amount, fee float64, bankCardNo string) (*Withdrawal, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	u := s.UserByID(userID)
	if u == nil {
		return nil, fmt.Errorf("用户不存在")
	}
	if u.Balance < amount+fee {
		return nil, fmt.Errorf("可提现余额不足")
	}
	w := &Withdrawal{
		ID:         randID("w"),
		UserID:     userID,
		Amount:     amount,
		Fee:        fee,
		Method:     method,
		Account:    account,
		BankCardNo: bankCardNo,
		Status:     "processing",
		ApplyTime:  time.Now().UnixMilli(),
	}
	now := time.Now().UnixMilli()
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(
		`INSERT INTO withdrawals (id, user_id, amount, fee, method, account, bank_card_no, status, apply_time, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		w.ID, userID, amount, fee, method, account, bankCardNo, w.Status, w.ApplyTime, now); err != nil {
		tx.Rollback()
		return nil, err
	}
	// 扣减余额（内存；事务提交后持久化）
	s.mu.Lock()
	u.Balance = round2(u.Balance - amount - fee)
	s.mu.Unlock()
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	s.syncUser(u)
	return w, nil
}

// ListWithdrawals 提现记录（用户视角）
func (s *Store) ListWithdrawals(userID string) ([]Withdrawal, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	rows, err := db.Query(`SELECT id, user_id, amount, fee, method, account, bank_card_no, status, apply_time, finish_time, remark FROM withdrawals WHERE user_id = ? ORDER BY apply_time DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Withdrawal{}
	for rows.Next() {
		var w Withdrawal
		var finishTime sql.NullInt64
		if rows.Scan(&w.ID, &w.UserID, &w.Amount, &w.Fee, &w.Method, &w.Account, &w.BankCardNo, &w.Status, &w.ApplyTime, &finishTime, &w.Remark) != nil {
			continue
		}
		w.FinishTime = nullInt64(finishTime)
		out = append(out, w)
	}
	return out, rows.Err()
}

// ListAllWithdrawals 全部提现申请（管理端）
func (s *Store) ListAllWithdrawals() ([]Withdrawal, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	rows, err := db.Query(`SELECT id, user_id, amount, fee, method, account, bank_card_no, status, apply_time, finish_time, remark FROM withdrawals ORDER BY apply_time DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Withdrawal{}
	for rows.Next() {
		var w Withdrawal
		var finishTime sql.NullInt64
		if rows.Scan(&w.ID, &w.UserID, &w.Amount, &w.Fee, &w.Method, &w.Account, &w.BankCardNo, &w.Status, &w.ApplyTime, &finishTime, &w.Remark) != nil {
			continue
		}
		w.FinishTime = nullInt64(finishTime)
		out = append(out, w)
	}
	return out, rows.Err()
}

// GetWithdrawal 查询提现申请（管理端）
func (s *Store) GetWithdrawal(id string) (*Withdrawal, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	var w Withdrawal
	var finishTime sql.NullInt64
	err := db.QueryRow(
		`SELECT id, user_id, amount, fee, method, account, bank_card_no, status, apply_time, finish_time, remark
         FROM withdrawals WHERE id = ?`, id).
		Scan(&w.ID, &w.UserID, &w.Amount, &w.Fee, &w.Method, &w.Account, &w.BankCardNo, &w.Status, &w.ApplyTime, &finishTime, &w.Remark)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	w.FinishTime = nullInt64(finishTime)
	return &w, nil
}

// CompleteWithdrawal 提现打款完成（status=done；failed 时退款到余额）
func (s *Store) CompleteWithdrawal(id, status string) (*Withdrawal, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	w, err := s.GetWithdrawal(id)
	if err != nil {
		return nil, err
	}
	if w == nil {
		return nil, fmt.Errorf("提现申请不存在")
	}
	if w.Status != "processing" {
		return nil, fmt.Errorf("该申请已处理")
	}
	now := time.Now().UnixMilli()
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`UPDATE withdrawals SET status = ?, finish_time = ? WHERE id = ?`, status, now, id); err != nil {
		tx.Rollback()
		return nil, err
	}
	// failed：退款到余额（内存扣改；事务提交后持久化，避免占用唯一连接）
	var refund *User
	if status == "failed" {
		if u := s.UserByID(w.UserID); u != nil {
			s.mu.Lock()
			u.Balance = round2(u.Balance + w.Amount + w.Fee)
			s.mu.Unlock()
			refund = u
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if refund != nil {
		s.syncUser(refund)
	}
	w.Status = status
	w.FinishTime = now
	return w, nil
}

// ==================== 工具 ====================

func nullString(n sql.NullString) string {
	if n.Valid {
		return n.String
	}
	return ""
}

func nullInt64(n sql.NullInt64) int64 {
	if n.Valid {
		return n.Int64
	}
	return 0
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}

func genOrderNo() string {
	now := time.Now()
	return fmt.Sprintf("%s%04d", now.Format("20060102150405"), now.UnixNano()%10000)
}

