// team.go
// 团队存储（SQLite 持久化）
// 团队功能：邀请人数 > 2 或所在团队经营金额 > 1w 时可创建团队；
// 团队可发布服务类商品（服务来源=团队名），经营金额由团队服务商品订单累计。
package store

import (
	"database/sql"
	"fmt"
	"time"
)

// TeamMember 团队成员
type TeamMember struct {
	Phone    string `json:"phone"`
	NickName string `json:"nickName"`
	JoinTime int64  `json:"joinTime"`
}

// Team 团队
type Team struct {
	ID                string       `json:"id"`
	Name              string       `json:"name"`
	OwnerPhone        string       `json:"ownerPhone"`
	OwnerName         string       `json:"ownerName"`
	BusinessAmount    float64      `json:"businessAmount"` // 团队经营金额（元）
	Treasury          float64      `json:"treasury"`       // 团队金库（服务订单 90% 分成收入，仅团长可支配）
	SupportMemberPhone string      `json:"supportMemberPhone"` // 团队指定客服成员（接收/回复团队服务会话）
	CreatedAt         int64        `json:"createdAt"`
	Members           []TeamMember `json:"members,omitempty"`
}

// TreasuryLog 团队金库流水
type TreasuryLog struct {
	ID         string  `json:"id"`
	TeamID     string  `json:"teamId"`
	Type       string  `json:"type"` // income=服务分成收入 / withdraw=团长提取 / transfer=团长向成员转账
	Amount     float64 `json:"amount"`
	TargetPhone string `json:"targetPhone,omitempty"`
	TargetName string  `json:"targetName,omitempty"`
	Remark     string  `json:"remark"`
	CreatedAt  int64   `json:"createdAt"`
}

// CreateTeam 创建团队（调用方需先校验建团资格）
func (s *Store) CreateTeam(name, ownerPhone, ownerName string) (*Team, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	name = trimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("团队名称不能为空")
	}
	// 同一用户只能在一个团队（作为队长或成员）
	var cnt int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM team_members WHERE phone = ?`, ownerPhone).Scan(&cnt); err != nil {
		return nil, err
	}
	if cnt > 0 {
		return nil, fmt.Errorf("您已在一个团队中，请先退出原团队")
	}
	t := &Team{
		ID:         randID("t"),
		Name:       name,
		OwnerPhone: ownerPhone,
		OwnerName:  ownerName,
		CreatedAt:  time.Now().UnixMilli(),
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(
		`INSERT INTO teams (id, name, owner_phone, owner_name, business_amount, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		t.ID, t.Name, t.OwnerPhone, t.OwnerName, 0, t.CreatedAt); err != nil {
		tx.Rollback()
		return nil, err
	}
	if _, err := tx.Exec(
		`INSERT INTO team_members (team_id, phone, nick_name, join_time) VALUES (?, ?, ?, ?)`,
		t.ID, ownerPhone, ownerName, t.CreatedAt); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	t.Members = []TeamMember{{Phone: ownerPhone, NickName: ownerName, JoinTime: t.CreatedAt}}
	return t, nil
}

// GetTeamByPhone 查询用户（手机号）所在团队
func (s *Store) GetTeamByPhone(phone string) (*Team, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	phone = trimSpace(phone)
	if phone == "" {
		return nil, nil
	}
	var teamID string
	err := db.QueryRow(
		`SELECT team_id FROM team_members WHERE phone = ? LIMIT 1`, phone).Scan(&teamID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.getTeamFull(teamID)
}

// JoinTeam 加入团队成为成员（需不在任何团队中）
func (s *Store) JoinTeam(teamID string, member *User) (*Team, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	if member == nil {
		return nil, fmt.Errorf("请先登录")
	}
	// 已在团队中不可再加入
	cur, err := s.GetTeamByPhone(member.Phone)
	if err != nil {
		return nil, err
	}
	if cur != nil {
		return nil, fmt.Errorf("您已在团队「%s」中，不可再加入其他团队", cur.Name)
	}
	team, err := s.GetTeam(teamID)
	if err != nil {
		return nil, err
	}
	if team == nil {
		return nil, fmt.Errorf("团队不存在")
	}
	_, err = db.Exec(
		`INSERT INTO team_members (team_id, phone, nick_name, join_time) VALUES (?, ?, ?, ?)`,
		team.ID, member.Phone, member.NickName, time.Now().UnixMilli())
	if err != nil {
		return nil, err
	}
	return s.GetTeam(team.ID)
}

// GetTeam 按 ID 查询团队
func (s *Store) GetTeam(id string) (*Team, error) {
	return s.getTeamFull(id)
}

// GetTeamByName 按团队名称查询团队（客服分配：团队服务会话分配给该团队指定客服成员）
func (s *Store) GetTeamByName(name string) (*Team, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	name = trimSpace(name)
	if name == "" {
		return nil, nil
	}
	var id string
	err := db.QueryRow(`SELECT id FROM teams WHERE name = ? LIMIT 1`, name).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.getTeamFull(id)
}

// SetTeamSupportMember 指定团队的客服成员（手机号须为该团队成员）。
// 团队服务类商品的客服会话将分配给该成员接收与回复。
func (s *Store) SetTeamSupportMember(teamID, memberPhone string) (*Team, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	team, err := s.getTeamFull(teamID)
	if err != nil {
		return nil, err
	}
	if team == nil {
		return nil, fmt.Errorf("团队不存在")
	}
	memberPhone = trimSpace(memberPhone)
	if memberPhone == "" {
		return nil, fmt.Errorf("请选择客服成员")
	}
	// 校验成员在团队中
	okMember := false
	for _, m := range team.Members {
		if m.Phone == memberPhone {
			okMember = true
			break
		}
	}
	if !okMember {
		return nil, fmt.Errorf("该成员不在团队中")
	}
	if _, err := db.Exec(`UPDATE teams SET support_member_phone = ? WHERE id = ?`, memberPhone, teamID); err != nil {
		return nil, err
	}
	team.SupportMemberPhone = memberPhone
	return team, nil
}

func (s *Store) getTeamFull(id string) (*Team, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	var t Team
	err := db.QueryRow(
		`SELECT id, name, owner_phone, owner_name, business_amount, treasury, support_member_phone, created_at FROM teams WHERE id = ?`, id).
		Scan(&t.ID, &t.Name, &t.OwnerPhone, &t.OwnerName, &t.BusinessAmount, &t.Treasury, &t.SupportMemberPhone, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(
		`SELECT phone, nick_name, join_time FROM team_members WHERE team_id = ? ORDER BY join_time`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	t.Members = []TeamMember{}
	for rows.Next() {
		var m TeamMember
		if rows.Scan(&m.Phone, &m.NickName, &m.JoinTime) == nil {
			t.Members = append(t.Members, m)
		}
	}
	return &t, rows.Err()
}

// AddTeamBusiness 为指定用户所在团队增加经营金额（服务订单）
func (s *Store) AddTeamBusiness(phone string, amount float64) error {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return fmt.Errorf("数据库未连接")
	}
	team, err := s.GetTeamByPhone(phone)
	if err != nil {
		return err
	}
	if team == nil {
		return fmt.Errorf("您不在任何团队中")
	}
	_, err = db.Exec(`UPDATE teams SET business_amount = business_amount + ? WHERE id = ?`, amount, team.ID)
	return err
}

// AddTeamBusinessByName 按团队名称累计经营金额（服务商品成交后，按来源团队名上报）
func (s *Store) AddTeamBusinessByName(teamName string, amount float64) error {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return fmt.Errorf("数据库未连接")
	}
	teamName = trimSpace(teamName)
	if teamName == "" {
		return fmt.Errorf("来源团队为空")
	}
	var id string
	err := db.QueryRow(`SELECT id FROM teams WHERE name = ?`, teamName).Scan(&id)
	if err == sql.ErrNoRows {
		return fmt.Errorf("团队「%s」不存在", teamName)
	}
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE teams SET business_amount = business_amount + ? WHERE id = ?`, amount, id)
	return err
}

// AddTeamServiceRevenueByName 团队服务订单分成入账（支付成功后按来源团队名上报）：
//   - 经营金额 business_amount 按订单全款累计（用于建团资格判定）
//   - 团队金库 treasury 按订单金额 90% 入账（平台抽成 10%）
//   - 记录金库收入流水
func (s *Store) AddTeamServiceRevenueByName(teamName string, total float64) error {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return fmt.Errorf("数据库未连接")
	}
	teamName = trimSpace(teamName)
	if teamName == "" {
		return fmt.Errorf("来源团队为空")
	}
	var id string
	err := db.QueryRow(`SELECT id FROM teams WHERE name = ?`, teamName).Scan(&id)
	if err == sql.ErrNoRows {
		return fmt.Errorf("团队「%s」不存在", teamName)
	}
	if err != nil {
		return err
	}
	treasuryIn := round2(total * 0.9) // 平台抽成 10%，剩余 90% 入金库
	now := time.Now().UnixMilli()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(
		`UPDATE teams SET business_amount = business_amount + ?, treasury = treasury + ? WHERE id = ?`,
		total, treasuryIn, id); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO team_treasury_logs (id, team_id, type, amount, remark, created_at) VALUES (?, ?, 'income', ?, ?, ?)`,
		randID("tl"), id, treasuryIn, "服务订单分成收入（平台抽成 10%）", now); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// WithdrawTreasury 团长提取团队金库到自己的余额（金库唯一支配人=团长）。
// 金库扣减在事务内原子执行（余额不足失败）；用户余额在事务提交后修改内存并持久化，
// 避免事务内嵌套查询占用唯一 SQLite 连接导致死锁。
func (s *Store) WithdrawTreasury(team *Team, ownerUser *User, amount float64) (*User, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	if team == nil || ownerUser == nil {
		return nil, fmt.Errorf("团队或用户不存在")
	}
	if ownerUser.Phone != team.OwnerPhone {
		return nil, fmt.Errorf("仅有团长可以提取团队金库")
	}
	if amount <= 0 {
		return nil, fmt.Errorf("提取金额无效")
	}
	now := time.Now().UnixMilli()
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	res, err := tx.Exec(
		`UPDATE teams SET treasury = treasury - ? WHERE id = ? AND treasury >= ?`,
		amount, team.ID, amount)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		tx.Rollback()
		return nil, fmt.Errorf("金库余额不足")
	}
	if _, err := tx.Exec(
		`INSERT INTO team_treasury_logs (id, team_id, type, amount, target_phone, target_name, remark, created_at)
         VALUES (?, ?, 'withdraw', ?, ?, ?, '团长提取到余额', ?)`,
		randID("tl"), team.ID, amount, ownerUser.Phone, ownerUser.NickName, now); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	ownerUser.Balance = round2(ownerUser.Balance + amount)
	s.mu.Unlock()
	s.syncUser(ownerUser)
	return ownerUser, nil
}

// TransferTreasury 团长从金库直接向团队成员余额转账。
// 目标须为团队成员且非团长本人；金额与成员校验后事务内扣减金库，提交后修改成员余额。
func (s *Store) TransferTreasury(team *Team, ownerUser *User, targetPhone string, amount float64) (*User, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	if team == nil || ownerUser == nil {
		return nil, fmt.Errorf("团队或用户不存在")
	}
	if ownerUser.Phone != team.OwnerPhone {
		return nil, fmt.Errorf("仅有团长可以操作团队金库")
	}
	targetPhone = trimSpace(targetPhone)
	if targetPhone == "" {
		return nil, fmt.Errorf("请选择转账成员")
	}
	if targetPhone == team.OwnerPhone {
		return nil, fmt.Errorf("请使用「提取到我的余额」将金库资金转入团长余额")
	}
	// 校验目标成员在团队中（事务外查询，避免占用唯一连接）
	targetName := ""
	inTeam := false
	for _, m := range team.Members {
		if m.Phone == targetPhone {
			inTeam = true
			targetName = m.NickName
			break
		}
	}
	if !inTeam {
		return nil, fmt.Errorf("转账目标不是团队成员")
	}
	target := s.UserByPhone(targetPhone)
	if target == nil {
		return nil, fmt.Errorf("目标成员「%s」尚未注册，无法转账", targetPhone)
	}
	if amount <= 0 {
		return nil, fmt.Errorf("转账金额无效")
	}
	now := time.Now().UnixMilli()
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	res, err := tx.Exec(
		`UPDATE teams SET treasury = treasury - ? WHERE id = ? AND treasury >= ?`,
		amount, team.ID, amount)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		tx.Rollback()
		return nil, fmt.Errorf("金库余额不足")
	}
	if _, err := tx.Exec(
		`INSERT INTO team_treasury_logs (id, team_id, type, amount, target_phone, target_name, remark, created_at)
         VALUES (?, ?, 'transfer', ?, ?, ?, '团长向成员转账', ?)`,
		randID("tl"), team.ID, amount, targetPhone, targetName, now); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	target.Balance = round2(target.Balance + amount)
	s.mu.Unlock()
	s.syncUser(target)
	return target, nil
}

// ListTreasuryLogs 团队金库流水（按时间倒序）
func (s *Store) ListTreasuryLogs(teamID string) ([]TreasuryLog, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	rows, err := db.Query(
		`SELECT id, team_id, type, amount, target_phone, target_name, remark, created_at
         FROM team_treasury_logs WHERE team_id = ? ORDER BY created_at DESC`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TreasuryLog{}
	for rows.Next() {
		var l TreasuryLog
		var targetPhone, targetName sql.NullString
		if rows.Scan(&l.ID, &l.TeamID, &l.Type, &l.Amount, &targetPhone, &targetName, &l.Remark, &l.CreatedAt) != nil {
			continue
		}
		l.TargetPhone = nullString(targetPhone)
		l.TargetName = nullString(targetName)
		out = append(out, l)
	}
	return out, rows.Err()
}

// ListTeams 全部团队（含成员，管理端用）
func (s *Store) ListTeams() ([]Team, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	rows, err := db.Query(`SELECT id, name, owner_phone, owner_name, business_amount, treasury, support_member_phone, created_at FROM teams ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	out := []Team{}
	for rows.Next() {
		var t Team
		if rows.Scan(&t.ID, &t.Name, &t.OwnerPhone, &t.OwnerName, &t.BusinessAmount, &t.Treasury, &t.SupportMemberPhone, &t.CreatedAt) != nil {
			continue
		}
		out = append(out, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// 先关闭游标再逐团队查成员，避免 SQLite 单连接死锁
	for i := range out {
		mrows, err := db.Query(
			`SELECT phone, nick_name, join_time FROM team_members WHERE team_id = ? ORDER BY join_time`, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Members = []TeamMember{}
		for mrows.Next() {
			var m TeamMember
			if mrows.Scan(&m.Phone, &m.NickName, &m.JoinTime) == nil {
				out[i].Members = append(out[i].Members, m)
			}
		}
		mrows.Close()
	}
	return out, nil
}

// CountTeams 团队数量
func (s *Store) CountTeams() int {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return 0
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM teams`).Scan(&n)
	return n
}

// DeleteTeam 删除团队（连同成员关系）
func (s *Store) DeleteTeam(id string) error {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return fmt.Errorf("数据库未连接")
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM team_members WHERE team_id = ?`, id); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.Exec(`DELETE FROM teams WHERE id = ?`, id); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n') {
		end--
	}
	return s[start:end]
}

// ---------- 团队创建申请（团员建新团需团长审核） ----------

// TeamCreateRequest 团员创建新团申请
type TeamCreateRequest struct {
	ID              string `json:"id"`
	RequesterPhone  string `json:"requesterPhone"`
	RequesterName   string `json:"requesterName"`
	TeamName        string `json:"teamName"`
	CurrentTeamID   string `json:"currentTeamId"`
	CurrentTeamName string `json:"currentTeamName"`
	Status          string `json:"status"` // pending / approved / rejected
	CreatedAt       int64  `json:"createdAt"`
	ReviewedAt      int64  `json:"reviewedAt,omitempty"`
	ReviewerPhone   string `json:"reviewerPhone,omitempty"`
}

// CreateTeamRequest 提交建团申请（发起人须是某团队成员）
func (s *Store) CreateTeamRequest(requester *User, teamName string, currentTeam *Team) (*TeamCreateRequest, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	teamName = trimSpace(teamName)
	if teamName == "" {
		return nil, fmt.Errorf("团队名称不能为空")
	}
	if currentTeam == nil {
		return nil, fmt.Errorf("仅团队成员可提交建团申请")
	}
	// 已有待审核申请则不能重复提交
	var cnt int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM team_create_requests WHERE requester_phone = ? AND status = 'pending'`,
		requester.Phone).Scan(&cnt); err != nil {
		return nil, err
	}
	if cnt > 0 {
		return nil, fmt.Errorf("已有待审核的建团申请，请等待团长审核")
	}
	req := &TeamCreateRequest{
		ID:              randID("tr"),
		RequesterPhone:  requester.Phone,
		RequesterName:   requester.NickName,
		TeamName:        teamName,
		CurrentTeamID:   currentTeam.ID,
		CurrentTeamName: currentTeam.Name,
		Status:          "pending",
		CreatedAt:       time.Now().UnixMilli(),
	}
	_, err := db.Exec(
		`INSERT INTO team_create_requests (id, requester_phone, requester_name, team_name, current_team_id, current_team_name, status, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		req.ID, req.RequesterPhone, req.RequesterName, req.TeamName, req.CurrentTeamID, req.CurrentTeamName, req.Status, req.CreatedAt)
	return req, err
}

// ListTeamRequestsByRequester 我提交的建团申请
func (s *Store) ListTeamRequestsByRequester(phone string) ([]TeamCreateRequest, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	return s.scanTeamRequests(`WHERE requester_phone = ? ORDER BY created_at DESC`, phone)
}

// ListPendingTeamRequestsForTeam 指定团队的待审核申请（团长收件箱）
func (s *Store) ListPendingTeamRequestsForTeam(teamID string) ([]TeamCreateRequest, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	return s.scanTeamRequests(`WHERE current_team_id = ? AND status = 'pending' ORDER BY created_at DESC`, teamID)
}

func (s *Store) scanTeamRequests(where string, args ...interface{}) ([]TeamCreateRequest, error) {
	rows, err := s.db.Query(
		`SELECT id, requester_phone, requester_name, team_name, current_team_id, current_team_name, status, created_at, reviewed_at, reviewer_phone FROM team_create_requests `+where,
		args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TeamCreateRequest{}
	for rows.Next() {
		var r TeamCreateRequest
		var reviewedAt sql.NullInt64
		var reviewer sql.NullString
		if rows.Scan(&r.ID, &r.RequesterPhone, &r.RequesterName, &r.TeamName,
			&r.CurrentTeamID, &r.CurrentTeamName, &r.Status, &r.CreatedAt, &reviewedAt, &reviewer) != nil {
			continue
		}
		if reviewedAt.Valid {
			r.ReviewedAt = reviewedAt.Int64
		}
		if reviewer.Valid {
			r.ReviewerPhone = reviewer.String
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetTeamRequest 查询建团申请
func (s *Store) GetTeamRequest(id string) (*TeamCreateRequest, error) {
	reqs, err := s.scanTeamRequests(`WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	if len(reqs) == 0 {
		return nil, nil
	}
	return &reqs[0], nil
}

// ApproveTeamRequest 团长审核通过：为该成员创建新团队（成员从原团队移出），并标记申请已通过
func (s *Store) ApproveTeamRequest(id, reviewerPhone string) (*Team, *TeamCreateRequest, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return nil, nil, fmt.Errorf("数据库未连接")
	}
	req, err := s.GetTeamRequest(id)
	if err != nil {
		return nil, nil, err
	}
	if req == nil {
		return nil, nil, fmt.Errorf("申请不存在")
	}
	if req.Status != "pending" {
		return nil, nil, fmt.Errorf("该申请已处理")
	}
	// 校验申请人在原团队中（防止原团队已解散/已移出）
	current, err := s.GetTeam(req.CurrentTeamID)
	if err != nil {
		return nil, nil, err
	}
	if current == nil {
		return nil, nil, fmt.Errorf("原团队已不存在")
	}
	isMember := false
	for _, m := range current.Members {
		if m.Phone == req.RequesterPhone {
			isMember = true
			break
		}
	}
	if !isMember {
		return nil, nil, fmt.Errorf("申请人已不在原团队中")
	}
	// 先把申请人从原团队移出，再创建新团队（CreateTeam 要求用户不在任何团队中）
	if _, err := db.Exec(`DELETE FROM team_members WHERE team_id = ? AND phone = ?`, req.CurrentTeamID, req.RequesterPhone); err != nil {
		return nil, nil, err
	}
	team, err := s.CreateTeam(req.TeamName, req.RequesterPhone, req.RequesterName)
	if err != nil {
		return nil, nil, err
	}
	if _, err := db.Exec(
		`UPDATE team_create_requests SET status = 'approved', reviewed_at = ?, reviewer_phone = ? WHERE id = ?`,
		time.Now().UnixMilli(), reviewerPhone, req.ID); err != nil {
		return nil, nil, err
	}
	req.Status = "approved"
	req.ReviewedAt = time.Now().UnixMilli()
	req.ReviewerPhone = reviewerPhone
	return team, req, nil
}

// RejectTeamRequest 团长审核驳回
func (s *Store) RejectTeamRequest(id, reviewerPhone string) (*TeamCreateRequest, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	req, err := s.GetTeamRequest(id)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, fmt.Errorf("申请不存在")
	}
	if req.Status != "pending" {
		return nil, fmt.Errorf("该申请已处理")
	}
	_, err = db.Exec(
		`UPDATE team_create_requests SET status = 'rejected', reviewed_at = ?, reviewer_phone = ? WHERE id = ?`,
		time.Now().UnixMilli(), reviewerPhone, req.ID)
	if err != nil {
		return nil, err
	}
	req.Status = "rejected"
	req.ReviewedAt = time.Now().UnixMilli()
	req.ReviewerPhone = reviewerPhone
	return req, nil
}

// ==================== 团队入团邀请（团长邀请 → 对方同意后入团） ====================

// TeamInvite 团队入团邀请
type TeamInvite struct {
	ID           string `json:"id"`
	TeamID       string `json:"teamId"`
	TeamName     string `json:"teamName"`
	InviterPhone string `json:"inviterPhone"`
	InviterName  string `json:"inviterName"`
	InviteePhone string `json:"inviteePhone"`
	InviteeName  string `json:"inviteeName"`
	Status       string `json:"status"` // pending / accepted / rejected / cancelled
	CreatedAt    int64  `json:"createdAt"`
	RespondedAt  int64  `json:"respondedAt,omitempty"`
}

// CreateTeamInvite 创建团队入团邀请（调用方已校验团长身份与邀请人信息）
// 规则：被邀请人必须已注册、不在任何团队中、且无待处理的重复邀请；重复邀请幂等返回已存在项。
func (s *Store) CreateTeamInvite(team *Team, inviter *User, invitee *User) (*TeamInvite, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	if team == nil || inviter == nil || invitee == nil {
		return nil, fmt.Errorf("参数错误")
	}
	// 已在团队中不可邀请（一个用户只能在一个团队）
	cur, err := s.GetTeamByPhone(invitee.Phone)
	if err != nil {
		return nil, err
	}
	if cur != nil {
		if cur.ID == team.ID {
			return nil, fmt.Errorf("「%s」已是团队成员", invitee.NickName)
		}
		return nil, fmt.Errorf("「%s」已在团队「%s」中，无法邀请", invitee.NickName, cur.Name)
	}
	// 重复待处理邀请：幂等返回已存在项
	var existing string
	err = db.QueryRow(
		`SELECT id FROM team_invites WHERE team_id = ? AND invitee_phone = ? AND status = 'pending' LIMIT 1`,
		team.ID, invitee.Phone).Scan(&existing)
	if err == nil {
		return s.GetTeamInvite(existing)
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	inv := &TeamInvite{
		ID:           randID("ti"),
		TeamID:       team.ID,
		TeamName:     team.Name,
		InviterPhone: inviter.Phone,
		InviterName:  inviter.NickName,
		InviteePhone: invitee.Phone,
		InviteeName:  invitee.NickName,
		Status:       "pending",
		CreatedAt:    time.Now().UnixMilli(),
	}
	if _, err := db.Exec(
		`INSERT INTO team_invites (id, team_id, team_name, inviter_phone, inviter_name, invitee_phone, invitee_name, status, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', ?)`,
		inv.ID, inv.TeamID, inv.TeamName, inv.InviterPhone, inv.InviterName, inv.InviteePhone, inv.InviteeName, inv.CreatedAt); err != nil {
		return nil, err
	}
	return inv, nil
}

// GetTeamInvite 查询邀请
func (s *Store) GetTeamInvite(id string) (*TeamInvite, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	var inv TeamInvite
	var respondedAt sql.NullInt64
	err := db.QueryRow(
		`SELECT id, team_id, team_name, inviter_phone, inviter_name, invitee_phone, invitee_name, status, created_at, responded_at
         FROM team_invites WHERE id = ?`, id).
		Scan(&inv.ID, &inv.TeamID, &inv.TeamName, &inv.InviterPhone, &inv.InviterName, &inv.InviteePhone, &inv.InviteeName, &inv.Status, &inv.CreatedAt, &respondedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if respondedAt.Valid {
		inv.RespondedAt = respondedAt.Int64
	}
	return &inv, nil
}



// HasPendingTeamInvite 指定团队对某手机号是否已有待处理邀请
func (s *Store) HasPendingTeamInvite(teamID, phone string) (bool, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return false, fmt.Errorf("数据库未连接")
	}
	var cnt int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM team_invites WHERE team_id = ? AND invitee_phone = ? AND status = 'pending'`,
		teamID, phone).Scan(&cnt)
	if err != nil {
		return false, err
	}
	return cnt > 0, nil
}



// ListTeamInvites 团队发出的全部邀请（团长收件箱，含历史状态，按创建时间倒序）
func (s *Store) ListTeamInvites(teamID string) ([]TeamInvite, error) {
	return s.scanTeamInvites(`WHERE team_id = ? ORDER BY created_at DESC`, teamID)
}

// ListTeamInviteInbox 我收到的待处理邀请（被邀请人视角，按创建时间倒序）
func (s *Store) ListTeamInviteInbox(phone string) ([]TeamInvite, error) {
	return s.scanTeamInvites(`WHERE invitee_phone = ? AND status = 'pending' ORDER BY created_at DESC`, phone)
}

func (s *Store) scanTeamInvites(where string, args ...interface{}) ([]TeamInvite, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	rows, err := db.Query(
		`SELECT id, team_id, team_name, inviter_phone, inviter_name, invitee_phone, invitee_name, status, created_at, responded_at
         FROM team_invites `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TeamInvite{}
	for rows.Next() {
		var inv TeamInvite
		var respondedAt sql.NullInt64
		if rows.Scan(&inv.ID, &inv.TeamID, &inv.TeamName, &inv.InviterPhone, &inv.InviterName, &inv.InviteePhone, &inv.InviteeName, &inv.Status, &inv.CreatedAt, &respondedAt) != nil {
			continue
		}
		if respondedAt.Valid {
			inv.RespondedAt = respondedAt.Int64
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

// AcceptTeamInvite 接受邀请并入团（需不在任何团队中）
func (s *Store) AcceptTeamInvite(id string, user *User) (*TeamInvite, *Team, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return nil, nil, fmt.Errorf("数据库未连接")
	}
	if user == nil {
		return nil, nil, fmt.Errorf("请先登录")
	}
	inv, err := s.GetTeamInvite(id)
	if err != nil {
		return nil, nil, err
	}
	if inv == nil {
		return nil, nil, fmt.Errorf("邀请不存在")
	}
	if inv.Status != "pending" {
		return nil, nil, fmt.Errorf("该邀请已处理（%s）", inviteStatusText(inv.Status))
	}
	if inv.InviteePhone != user.Phone {
		return nil, nil, fmt.Errorf("该邀请不是发给您的")
	}
	team, err := s.JoinTeam(inv.TeamID, user)
	if err != nil {
		return nil, nil, err
	}
	if _, err := db.Exec(
		`UPDATE team_invites SET status = 'accepted', responded_at = ? WHERE id = ?`,
		time.Now().UnixMilli(), inv.ID); err != nil {
		return nil, nil, err
	}
	inv.Status = "accepted"
	inv.RespondedAt = time.Now().UnixMilli()
	return inv, team, nil
}

// RejectTeamInvite 拒绝邀请
func (s *Store) RejectTeamInvite(id, phone string) (*TeamInvite, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	inv, err := s.GetTeamInvite(id)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, fmt.Errorf("邀请不存在")
	}
	if inv.Status != "pending" {
		return nil, fmt.Errorf("该邀请已处理（%s）", inviteStatusText(inv.Status))
	}
	if inv.InviteePhone != phone {
		return nil, fmt.Errorf("该邀请不是发给您的")
	}
	if _, err := db.Exec(
		`UPDATE team_invites SET status = 'rejected', responded_at = ? WHERE id = ?`,
		time.Now().UnixMilli(), inv.ID); err != nil {
		return nil, err
	}
	inv.Status = "rejected"
	inv.RespondedAt = time.Now().UnixMilli()
	return inv, nil
}

// CancelTeamInvite 团长取消待处理邀请
func (s *Store) CancelTeamInvite(id, inviterPhone string) (*TeamInvite, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	inv, err := s.GetTeamInvite(id)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, fmt.Errorf("邀请不存在")
	}
	if inv.Status != "pending" {
		return nil, fmt.Errorf("该邀请已处理，无法取消")
	}
	if inv.InviterPhone != inviterPhone {
		return nil, fmt.Errorf("仅邀请人可取消该邀请")
	}
	if _, err := db.Exec(
		`UPDATE team_invites SET status = 'cancelled', responded_at = ? WHERE id = ?`,
		time.Now().UnixMilli(), inv.ID); err != nil {
		return nil, err
	}
	inv.Status = "cancelled"
	inv.RespondedAt = time.Now().UnixMilli()
	return inv, nil
}

func inviteStatusText(status string) string {
	switch status {
	case "accepted":
		return "已接受"
	case "rejected":
		return "已拒绝"
	case "cancelled":
		return "已取消"
	default:
		return "待处理"
	}
}
