// admin_account.go
// 管理后台账号（SQLite 持久化）：admin 超级管理员 / staff 员工
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// AdminAccount 管理后台账号
type AdminAccount struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"`   // admin / staff
	Status       int    `json:"status"` // 1 启用 / 0 停用
	CreatedAt    int64  `json:"createdAt"`
}

// HashPassword bcrypt 哈希密码
func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

// CheckPassword 校验密码（哈希比对）
func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

func (s *Store) getDB() *sql.DB {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	return db
}

// InitAdmin 初始化 admin 账号（无任何账号时创建）
func (s *Store) InitAdmin(username, password string) error {
	db := s.getDB()
	if db == nil {
		return fmt.Errorf("数据库未连接")
	}
	var cnt int
	if err := db.QueryRow(`SELECT COUNT(*) FROM admin_users`).Scan(&cnt); err != nil {
		return err
	}
	if cnt > 0 {
		return nil
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	_, err = db.Exec(
		`INSERT INTO admin_users (id, username, password_hash, role, status, created_at) VALUES (?, ?, ?, 'admin', 1, ?)`,
		randID("a"), username, hash, time.Now().UnixMilli())
	return err
}

// GetAdminAccountByUsername 按用户名查询账号
func (s *Store) GetAdminAccountByUsername(username string) (*AdminAccount, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	var a AdminAccount
	err := db.QueryRow(
		`SELECT id, username, password_hash, role, status, created_at FROM admin_users WHERE username = ?`, username).
		Scan(&a.ID, &a.Username, &a.PasswordHash, &a.Role, &a.Status, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// GetAdminAccountByID 按 ID 查询账号
func (s *Store) GetAdminAccountByID(id string) (*AdminAccount, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	var a AdminAccount
	err := db.QueryRow(
		`SELECT id, username, password_hash, role, status, created_at FROM admin_users WHERE id = ?`, id).
		Scan(&a.ID, &a.Username, &a.PasswordHash, &a.Role, &a.Status, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// ListAdminAccounts 账号列表（不含密码哈希）
func (s *Store) ListAdminAccounts() ([]AdminAccount, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	rows, err := db.Query(`SELECT id, username, role, status, created_at FROM admin_users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AdminAccount{}
	for rows.Next() {
		var a AdminAccount
		if rows.Scan(&a.ID, &a.Username, &a.Role, &a.Status, &a.CreatedAt) != nil {
			continue
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// CreateStaffAccount 创建员工账号
func (s *Store) CreateStaffAccount(username, password string) (*AdminAccount, error) {
	db := s.getDB()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	existing, err := s.GetAdminAccountByUsername(username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("用户名已存在")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	a := &AdminAccount{
		ID: randID("a"), Username: username, PasswordHash: hash,
		Role: "staff", Status: 1, CreatedAt: time.Now().UnixMilli(),
	}
	_, err = db.Exec(
		`INSERT INTO admin_users (id, username, password_hash, role, status, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		a.ID, a.Username, a.PasswordHash, a.Role, a.Status, a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// DeleteAdminAccount 删除账号（admin 自身保护由调用方保证）
func (s *Store) DeleteAdminAccount(id string) error {
	db := s.getDB()
	if db == nil {
		return fmt.Errorf("数据库未连接")
	}
	_, err := db.Exec(`DELETE FROM admin_users WHERE id = ?`, id)
	return err
}

// UpdateAccountRole 修改账号权限（admin 分配）
func (s *Store) UpdateAccountRole(id, role string) error {
	db := s.getDB()
	if db == nil {
		return fmt.Errorf("数据库未连接")
	}
	_, err := db.Exec(`UPDATE admin_users SET role = ? WHERE id = ?`, role, id)
	return err
}

// UpdateAccountStatus 启用/停用账号
func (s *Store) UpdateAccountStatus(id string, status int) error {
	db := s.getDB()
	if db == nil {
		return fmt.Errorf("数据库未连接")
	}
	_, err := db.Exec(`UPDATE admin_users SET status = ? WHERE id = ?`, status, id)
	return err
}
