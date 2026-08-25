// support.go
// 客服工单与消息存储（SQLite 持久化）
// 分配规则：
//   - 团队服务商品（service=true 且 sourceTeam 有值）→ 分配给该团队指定客服成员（teams.support_member_phone）
//   - 普通商品 / 官方服务 → 后台客服（assignee_type=admin）
package store

import (
	"fmt"
	"time"
)

// SupportTicket 客服会话（工单）
type SupportTicket struct {
	ID            string `json:"id"`
	UserID        string `json:"userId"`
	UserPhone     string `json:"userPhone"`
	UserName      string `json:"userName"`
	Subject       string `json:"subject"`
	ProductID     string `json:"productId"`
	ProductName   string `json:"productName"`
	Service       bool   `json:"service"`
	SourceTeam    string `json:"sourceTeam"`
	AssigneeType  string `json:"assigneeType"` // admin / team
	AssigneePhone string `json:"assigneePhone"`
	Status        string `json:"status"` // open / closed
	LastMessage   string `json:"lastMessage"`
	LastTime      int64  `json:"lastTime"`
	CreatedAt     int64  `json:"createdAt"`
}

// SupportMessage 客服消息
type SupportMessage struct {
	ID         string `json:"id"`
	TicketID   string `json:"ticketId"`
	SenderType string `json:"senderType"` // user / admin / team
	SenderID   string `json:"senderId"`
	SenderName string `json:"senderName"`
	Content    string `json:"content"`
	CreatedAt  int64  `json:"createdAt"`
	Read       bool   `json:"read"`
}

// CreateSupportTicket 创建客服会话（自动分配客服）并写入首条消息
// sourceTeam 非空时：优先分配给该团队指定客服成员；团队未指定客服或查不到 → 后台客服
func (s *Store) CreateSupportTicket(user *User, subject, productID, productName string,
	service bool, sourceTeam, firstMessage string) (*SupportTicket, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	if user == nil {
		return nil, fmt.Errorf("请先登录")
	}
	assigneeType := "admin"
	assigneePhone := ""
	if service && trimSpace(sourceTeam) != "" {
		if team, err := s.GetTeamByName(sourceTeam); err == nil && team != nil && team.SupportMemberPhone != "" {
			assigneeType = "team"
			assigneePhone = team.SupportMemberPhone
		}
	}
	now := time.Now().UnixMilli()
	t := &SupportTicket{
		ID:            randID("st"),
		UserID:        user.ID,
		UserPhone:     user.Phone,
		UserName:      user.NickName,
		Subject:       subject,
		ProductID:     productID,
		ProductName:   productName,
		Service:       service,
		SourceTeam:    sourceTeam,
		AssigneeType:  assigneeType,
		AssigneePhone: assigneePhone,
		Status:        "open",
		LastMessage:   firstMessage,
		LastTime:      now,
		CreatedAt:     now,
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(
		`INSERT INTO support_tickets (id, user_id, user_phone, user_name, subject, product_id, product_name,
             service, source_team, assignee_type, assignee_phone, status, last_message, last_time, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.UserID, t.UserPhone, t.UserName, t.Subject, t.ProductID, t.ProductName,
		boolInt(t.Service), t.SourceTeam, assigneeType, assigneePhone, t.Status, t.LastMessage, t.LastTime, now, now); err != nil {
		tx.Rollback()
		return nil, err
	}
	if trimSpace(firstMessage) != "" {
		if _, err := tx.Exec(
			`INSERT INTO support_messages (id, ticket_id, sender_type, sender_id, sender_name, content, created_at, read)
             VALUES (?, ?, 'user', ?, ?, ?, ?, 1)`,
			randID("sm"), t.ID, user.ID, user.NickName, firstMessage, now); err != nil {
			tx.Rollback()
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return t, nil
}

// ListSupportTicketsByUser 用户自己的客服会话（按最近消息倒序）
func (s *Store) ListSupportTicketsByUser(userID string) ([]SupportTicket, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	return s.scanSupportTickets(`WHERE user_id = ? ORDER BY last_time DESC`, userID)
}

// ListSupportTicketsByAssignee 团队客服成员收件箱（分配给我回复的会话）
func (s *Store) ListSupportTicketsByAssignee(phone string) ([]SupportTicket, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	return s.scanSupportTickets(`WHERE assignee_type = 'team' AND assignee_phone = ? AND status = 'open' ORDER BY last_time DESC`, phone)
}

// ListAllSupportTickets 全部客服会话（后台客服收件箱；status 空或 all 返回全部）
func (s *Store) ListAllSupportTickets(status string) ([]SupportTicket, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	if status != "" && status != "all" {
		return s.scanSupportTickets(`WHERE status = ? ORDER BY last_time DESC`, status)
	}
	return s.scanSupportTickets(`ORDER BY last_time DESC`)
}

func (s *Store) scanSupportTickets(where string, args ...interface{}) ([]SupportTicket, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	rows, err := db.Query(
		`SELECT id, user_id, user_phone, user_name, subject, product_id, product_name,
                service, source_team, assignee_type, assignee_phone, status, last_message, last_time, created_at
         FROM support_tickets `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SupportTicket{}
	for rows.Next() {
		var t SupportTicket
		var service int
		if rows.Scan(&t.ID, &t.UserID, &t.UserPhone, &t.UserName, &t.Subject, &t.ProductID, &t.ProductName,
			&service, &t.SourceTeam, &t.AssigneeType, &t.AssigneePhone, &t.Status, &t.LastMessage, &t.LastTime, &t.CreatedAt) != nil {
			continue
		}
		t.Service = service == 1
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetSupportTicket 查询客服会话
func (s *Store) GetSupportTicket(id string) (*SupportTicket, error) {
	tickets, err := s.scanSupportTickets(`WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	if len(tickets) == 0 {
		return nil, nil
	}
	return &tickets[0], nil
}

// SendSupportMessage 用户发送客服消息（校验会话归属）
func (s *Store) SendSupportMessage(userID, ticketID, content string) (*SupportMessage, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	t, err := s.GetSupportTicket(ticketID)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, fmt.Errorf("会话不存在")
	}
	if t.UserID != userID {
		return nil, fmt.Errorf("无权操作该会话")
	}
	if t.Status == "closed" {
		return nil, fmt.Errorf("会话已关闭，如需帮助请新建会话")
	}
	content = trimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("消息内容不能为空")
	}
	u := s.UserByID(userID)
	senderName := "用户"
	if u != nil {
		senderName = u.NickName
	}
	now := time.Now().UnixMilli()
	msg := &SupportMessage{
		ID:         randID("sm"),
		TicketID:   ticketID,
		SenderType: "user",
		SenderID:   userID,
		SenderName: senderName,
		Content:    content,
		CreatedAt:  now,
		Read:       true,
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(
		`INSERT INTO support_messages (id, ticket_id, sender_type, sender_id, sender_name, content, created_at, read)
         VALUES (?, ?, 'user', ?, ?, ?, ?, 1)`,
		msg.ID, ticketID, userID, senderName, content, now); err != nil {
		tx.Rollback()
		return nil, err
	}
	if _, err := tx.Exec(
		`UPDATE support_tickets SET last_message = ?, last_time = ?, updated_at = ? WHERE id = ?`,
		content, now, now, ticketID); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return msg, nil
}

// SendSupportReply 客服/后台回复消息（senderType: admin / team）
func (s *Store) SendSupportReply(ticketID, senderType, senderName, content string) (*SupportMessage, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	t, err := s.GetSupportTicket(ticketID)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, fmt.Errorf("会话不存在")
	}
	content = trimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("回复内容不能为空")
	}
	now := time.Now().UnixMilli()
	msg := &SupportMessage{
		ID:         randID("sm"),
		TicketID:   ticketID,
		SenderType: senderType,
		SenderName: senderName,
		Content:    content,
		CreatedAt:  now,
		Read:       false,
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(
		`INSERT INTO support_messages (id, ticket_id, sender_type, sender_id, sender_name, content, created_at, read)
         VALUES (?, ?, ?, '', ?, ?, ?, 0)`,
		msg.ID, ticketID, senderType, senderName, content, now); err != nil {
		tx.Rollback()
		return nil, err
	}
	if _, err := tx.Exec(
		`UPDATE support_tickets SET last_message = ?, last_time = ?, updated_at = ? WHERE id = ?`,
		content, now, now, ticketID); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return msg, nil
}

// ListSupportMessages 会话消息列表（按时间正序）
func (s *Store) ListSupportMessages(ticketID string) ([]SupportMessage, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	rows, err := db.Query(
		`SELECT id, ticket_id, sender_type, sender_id, sender_name, content, created_at, read
         FROM support_messages WHERE ticket_id = ? ORDER BY created_at ASC`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SupportMessage{}
	for rows.Next() {
		var m SupportMessage
		var read int
		if rows.Scan(&m.ID, &m.TicketID, &m.SenderType, &m.SenderID, &m.SenderName, &m.Content, &m.CreatedAt, &read) != nil {
			continue
		}
		m.Read = read == 1
		out = append(out, m)
	}
	return out, rows.Err()
}

// MarkSupportTicketRead 标记会话中发给指定发送方类型的消息为已读（用户读客服回复 / 客服读用户消息）
func (s *Store) MarkSupportTicketRead(ticketID string, byUser bool) error {
	db := s.getDB()
	if db == nil {
		return fmt.Errorf("数据库未连接")
	}
	if byUser {
		_, err := db.Exec(`UPDATE support_messages SET read = 1 WHERE ticket_id = ? AND sender_type != 'user'`, ticketID)
		return err
	}
	_, err := db.Exec(`UPDATE support_messages SET read = 1 WHERE ticket_id = ? AND sender_type = 'user'`, ticketID)
	return err
}

// CloseSupportTicket 关闭会话
func (s *Store) CloseSupportTicket(id string) (*SupportTicket, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	t, err := s.GetSupportTicket(id)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, fmt.Errorf("会话不存在")
	}
	if _, err := db.Exec(
		`UPDATE support_tickets SET status = 'closed', updated_at = ? WHERE id = ?`,
		time.Now().UnixMilli(), id); err != nil {
		return nil, err
	}
	t.Status = "closed"
	return t, nil
}

// CountAllSupportTickets 管理端统计
func (s *Store) CountAllSupportTickets() int {
	db := s.getDB()
	if db == nil {
		return 0
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM support_tickets`).Scan(&n)
	return n
}

