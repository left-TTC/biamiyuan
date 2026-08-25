// cart.go
// 用户购物车历史：SQLite 持久化（记录商品 id 与数量）
package store

import (
	"fmt"
	"time"
)

// CartItem 用户购物车中的一条商品记录（记录商品 id 与数量）
type CartItem struct {
	ProductID string `json:"productId"`
	Quantity  int    `json:"quantity"`
}

// UserCart 某个用户的购物车（管理端展示用）
type UserCart struct {
	UserID string     `json:"userId"`
	Items  []CartItem `json:"items"`
}

// SyncCart 全量覆盖写入某用户的购物车（小程序每次变更后整体同步）
func (s *Store) SyncCart(userID string, items []CartItem) error {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return fmt.Errorf("数据库未连接")
	}
	now := time.Now().UnixMilli()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM cart_items WHERE user_id = ?`, userID); err != nil {
		tx.Rollback()
		return err
	}
	for _, it := range items {
		if it.Quantity <= 0 {
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO cart_items (user_id, product_id, quantity, created_at, updated_at)
             VALUES (?, ?, ?, ?, ?)`,
			userID, it.ProductID, it.Quantity, now, now,
		); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// ListCartItems 获取某用户的购物车
func (s *Store) ListCartItems(userID string) ([]CartItem, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	rows, err := db.Query(
		`SELECT product_id, quantity FROM cart_items WHERE user_id = ? ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CartItem{}
	for rows.Next() {
		var it CartItem
		if rows.Scan(&it.ProductID, &it.Quantity) != nil {
			continue
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// UpdateCartItemQuantity 更新某用户购物车中商品的数量（0 表示删除该商品）
func (s *Store) UpdateCartItemQuantity(userID, productID string, quantity int) error {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return fmt.Errorf("数据库未连接")
	}
	if quantity <= 0 {
		_, err := db.Exec(`DELETE FROM cart_items WHERE user_id = ? AND product_id = ?`, userID, productID)
		return err
	}
	_, err := db.Exec(
		`INSERT INTO cart_items (user_id, product_id, quantity, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?)
         ON CONFLICT(user_id, product_id) DO UPDATE SET
            quantity=excluded.quantity, updated_at=excluded.updated_at`,
		userID, productID, quantity, time.Now().UnixMilli(), time.Now().UnixMilli(),
	)
	return err
}

// DeleteCartItem 删除某用户购物车中的一条商品
func (s *Store) DeleteCartItem(userID, productID string) error {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return fmt.Errorf("数据库未连接")
	}
	_, err := db.Exec(`DELETE FROM cart_items WHERE user_id = ? AND product_id = ?`, userID, productID)
	return err
}

// ListAllCarts 所有用户的购物车（管理端展示）
func (s *Store) ListAllCarts() ([]UserCart, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	rows, err := db.Query(
		`SELECT user_id, product_id, quantity FROM cart_items ORDER BY user_id, updated_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byUser := map[string][]CartItem{}
	order := []string{}
	for rows.Next() {
		var uid, pid string
		var qty int
		if rows.Scan(&uid, &pid, &qty) != nil {
			continue
		}
		if _, ok := byUser[uid]; !ok {
			order = append(order, uid)
		}
		byUser[uid] = append(byUser[uid], CartItem{ProductID: pid, Quantity: qty})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]UserCart, 0, len(order))
	for _, uid := range order {
		out = append(out, UserCart{UserID: uid, Items: byUser[uid]})
	}
	return out, nil
}

// CountCartItems 购物车记录总数（用于统计，可选）
func (s *Store) CountCartItems() int {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return 0
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM cart_items`).Scan(&n)
	return n
}
