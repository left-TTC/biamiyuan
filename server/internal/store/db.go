// db.go
// SQLite 数据库连接与持久化（商品修改审计、系统元信息）
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动（无 CGO）
)

// ProductEdit 商品修改审计记录
type ProductEdit struct {
	ID        string `json:"id"`
	ProductID string `json:"productId"`
	Field     string `json:"field"`
	OldValue  string `json:"oldValue"`
	NewValue  string `json:"newValue"`
	Operator  string `json:"operator"`
	CreatedAt int64  `json:"createdAt"`
}

// OpenDB 打开 SQLite 数据库并执行迁移（幂等）
func (s *Store) OpenDB(dbPath string) error {
	if dir := filepath.Dir(dbPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1) // SQLite 单写者
	if err := db.Ping(); err != nil {
		return err
	}

	stmts := []string{
		// 用户账号（SQLite 持久化，重启后恢复；订单/地址/提现绑定 user_id）
		`CREATE TABLE IF NOT EXISTS users (
            id               TEXT PRIMARY KEY,
            phone            TEXT NOT NULL UNIQUE,
            nick_name        TEXT DEFAULT '',
            avatar_url       TEXT DEFAULT '',
            role             TEXT DEFAULT 'member',
            balance          REAL DEFAULT 0,
            total_commission REAL DEFAULT 0,
            promoter_code    TEXT DEFAULT '',
            invited_by       TEXT DEFAULT '',
            invited_at       INTEGER DEFAULT 0,
            notify_enabled   INTEGER DEFAULT 0,
            openid           TEXT DEFAULT '',
            created_at       INTEGER NOT NULL
        )`,
		// 佣金结算（服务端持久化）：订单支付后生成待结算佣金，
		// 无理由退货期（COMMISSION_SETTLE_DAYS 天）满后才可到账；期内退款则取消
		`CREATE TABLE IF NOT EXISTS commissions (
            id         TEXT PRIMARY KEY,
            user_id    TEXT NOT NULL,     -- 获得佣金的用户（邀请人）
            order_id   TEXT DEFAULT '',   -- 关联订单（模拟佣金为空）
            amount     REAL NOT NULL,
            status     TEXT NOT NULL,     -- pending 待结算 / settled 已到账 / cancelled 已取消(退款)
            paid_at    INTEGER NOT NULL,  -- 订单支付时间
            settle_at  INTEGER NOT NULL,  -- 佣金到账时间 = 支付时间 + 无理由退货期
            settled_at INTEGER DEFAULT 0,
            created_at INTEGER NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_commissions_user ON commissions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_commissions_order ON commissions(order_id)`,
		`CREATE TABLE IF NOT EXISTS product_edits (
            id         TEXT PRIMARY KEY,
            product_id TEXT NOT NULL,
            field      TEXT NOT NULL,
            old_value  TEXT,
            new_value  TEXT,
            operator   TEXT,
            created_at INTEGER NOT NULL
        )`,
		`CREATE TABLE IF NOT EXISTS sys_info (
            key   TEXT PRIMARY KEY,
            value TEXT
        )`,
		`CREATE TABLE IF NOT EXISTS categories (
            id         TEXT PRIMARY KEY,
            name       TEXT NOT NULL,
            parent_id  TEXT DEFAULT '',
            sort       INTEGER DEFAULT 0,
            is_service INTEGER DEFAULT 0
        )`,
		`CREATE TABLE IF NOT EXISTS products (
            id             TEXT PRIMARY KEY,
            name           TEXT NOT NULL,
            desc           TEXT DEFAULT '',
            price          REAL NOT NULL,
            original_price REAL NOT NULL,
            emoji          TEXT DEFAULT '',
            colors         TEXT DEFAULT '[]',
            images         TEXT DEFAULT '[]',
            sales          INTEGER DEFAULT 0,
            category       TEXT DEFAULT '',
            tags           TEXT DEFAULT '[]',
            detail         TEXT DEFAULT '[]',
            attributes     TEXT DEFAULT '[]',
            service        INTEGER DEFAULT 0,
            source_team    TEXT DEFAULT '',
            updated_at     INTEGER
        )`,
		// 用户购物车历史（持久化，记录商品 id 与数量）
		`CREATE TABLE IF NOT EXISTS cart_items (
            user_id    TEXT NOT NULL,
            product_id TEXT NOT NULL,
            quantity   INTEGER NOT NULL DEFAULT 1,
            created_at INTEGER NOT NULL,
            updated_at INTEGER NOT NULL,
            PRIMARY KEY (user_id, product_id)
        )`,
		`CREATE TABLE IF NOT EXISTS admin_users (
            id            TEXT PRIMARY KEY,
            username      TEXT NOT NULL UNIQUE,
            password_hash TEXT NOT NULL,
            role          TEXT NOT NULL DEFAULT 'staff', -- admin / staff
            status        INTEGER NOT NULL DEFAULT 1,     -- 1 启用 / 0 停用
            created_at    INTEGER NOT NULL
        )`,
		// 团队（团队可发布服务类商品；经营金额用于建团资格判定；金库存放服务订单分成收入）
		`CREATE TABLE IF NOT EXISTS teams (
            id             TEXT PRIMARY KEY,
            name           TEXT NOT NULL,
            owner_phone    TEXT NOT NULL,
            owner_name     TEXT DEFAULT '',
            business_amount REAL DEFAULT 0,
            treasury       REAL DEFAULT 0,
            created_at     INTEGER NOT NULL
        )`,
		`CREATE TABLE IF NOT EXISTS team_members (
            team_id   TEXT NOT NULL,
            phone     TEXT NOT NULL,
            nick_name TEXT DEFAULT '',
            join_time INTEGER NOT NULL,
            PRIMARY KEY (team_id, phone)
        )`,
		// 团队金库流水（服务分成收入 / 团长提取 / 团长向成员转账）
		`CREATE TABLE IF NOT EXISTS team_treasury_logs (
            id           TEXT PRIMARY KEY,
            team_id      TEXT NOT NULL,
            type         TEXT NOT NULL, -- income / withdraw / transfer
            amount       REAL NOT NULL,
            target_phone TEXT DEFAULT '',
            target_name  TEXT DEFAULT '',
            remark       TEXT DEFAULT '',
            created_at   INTEGER NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_treasury_logs_team ON team_treasury_logs(team_id)`,
		// 团员创建新团申请（需现任团长审核）
		`CREATE TABLE IF NOT EXISTS team_create_requests (
            id                TEXT PRIMARY KEY,
            requester_phone   TEXT NOT NULL,
            requester_name    TEXT DEFAULT '',
            team_name         TEXT NOT NULL,
            current_team_id   TEXT NOT NULL,
            current_team_name TEXT DEFAULT '',
            status            TEXT NOT NULL DEFAULT 'pending', -- pending / approved / rejected
            created_at        INTEGER NOT NULL,
            reviewed_at       INTEGER,
            reviewer_phone    TEXT DEFAULT ''
        )`,
		// 团队入团邀请（团长邀请成员，对方同意后入团）
		`CREATE TABLE IF NOT EXISTS team_invites (
            id             TEXT PRIMARY KEY,
            team_id        TEXT NOT NULL,
            team_name      TEXT DEFAULT '',
            inviter_phone  TEXT NOT NULL,
            inviter_name   TEXT DEFAULT '',
            invitee_phone  TEXT NOT NULL,
            invitee_name   TEXT DEFAULT '',
            status         TEXT NOT NULL DEFAULT 'pending', -- pending / accepted / rejected / cancelled
            created_at     INTEGER NOT NULL,
            responded_at   INTEGER
        )`,
		`CREATE INDEX IF NOT EXISTS idx_team_invites_team ON team_invites(team_id)`,
		`CREATE INDEX IF NOT EXISTS idx_team_invites_invitee ON team_invites(invitee_phone)`,

		// 收货地址（绑定账号，存服务器）
		`CREATE TABLE IF NOT EXISTS addresses (
            id         TEXT PRIMARY KEY,
            user_id    TEXT NOT NULL,
            name       TEXT NOT NULL,
            phone      TEXT NOT NULL,
            region     TEXT NOT NULL,
            detail     TEXT NOT NULL,
            is_default INTEGER NOT NULL DEFAULT 0,
            created_at INTEGER NOT NULL,
            updated_at INTEGER NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_addresses_user ON addresses(user_id)`,
		// 订单（含支付与物流）
		`CREATE TABLE IF NOT EXISTS orders (
            id           TEXT PRIMARY KEY,
            order_no     TEXT NOT NULL,
            user_id      TEXT NOT NULL,
            status       TEXT NOT NULL DEFAULT 'pending', -- pending/paid/shipped/done/canceled/refunded
            total_amount REAL NOT NULL,
            address_json TEXT NOT NULL DEFAULT '{}',
            remark       TEXT DEFAULT '',
            pay_method   TEXT DEFAULT '',
            pay_time     INTEGER,
            transaction_id TEXT DEFAULT '',
            ship_company TEXT DEFAULT '',
            ship_no      TEXT DEFAULT '',
            ship_time    INTEGER,
            finish_time  INTEGER,
            cancel_time  INTEGER,
            refund_time  INTEGER,
            created_at   INTEGER NOT NULL,
            updated_at   INTEGER NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_orders_user ON orders(user_id)`,
		`CREATE TABLE IF NOT EXISTS order_items (
            id          TEXT PRIMARY KEY,
            order_id    TEXT NOT NULL,
            product_id  TEXT NOT NULL,
            name        TEXT NOT NULL,
            price       REAL NOT NULL,
            quantity    INTEGER NOT NULL,
            emoji       TEXT DEFAULT '',
            images      TEXT DEFAULT '[]',
            attrs       TEXT DEFAULT '[]',
            service     INTEGER DEFAULT 0,
            source_team TEXT DEFAULT '',
            created_at  INTEGER NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_order_items_order ON order_items(order_id)`,
		// 提现申请（绑定企业银行卡号在服务器处理）
		`CREATE TABLE IF NOT EXISTS withdrawals (
            id           TEXT PRIMARY KEY,
            user_id      TEXT NOT NULL,
            amount       REAL NOT NULL,
            fee          REAL NOT NULL DEFAULT 0,
            method       TEXT NOT NULL DEFAULT 'bank',
            account      TEXT DEFAULT '',
            bank_card_no TEXT DEFAULT '',
            status       TEXT NOT NULL DEFAULT 'processing', -- processing / done / failed
            apply_time   INTEGER NOT NULL,
            finish_time  INTEGER,
            remark       TEXT DEFAULT '',
            created_at   INTEGER NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_withdrawals_user ON withdrawals(user_id)`,
		// 客服会话（普通商品/官方服务 -> 后台客服；团队服务 -> 团队指定客服成员）
		`CREATE TABLE IF NOT EXISTS support_tickets (
            id             TEXT PRIMARY KEY,
            user_id        TEXT NOT NULL,
            user_phone     TEXT DEFAULT '',
            user_name      TEXT DEFAULT '',
            subject        TEXT DEFAULT '',
            product_id     TEXT DEFAULT '',
            product_name   TEXT DEFAULT '',
            service        INTEGER DEFAULT 0,
            source_team    TEXT DEFAULT '',
            assignee_type  TEXT NOT NULL DEFAULT 'admin', -- admin / team
            assignee_phone TEXT DEFAULT '',
            status         TEXT NOT NULL DEFAULT 'open', -- open / closed
            last_message   TEXT DEFAULT '',
            last_time      INTEGER,
            created_at     INTEGER NOT NULL,
            updated_at     INTEGER NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_support_tickets_user ON support_tickets(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_support_tickets_assignee ON support_tickets(assignee_phone)`,
		`CREATE TABLE IF NOT EXISTS support_messages (
            id          TEXT PRIMARY KEY,
            ticket_id   TEXT NOT NULL,
            sender_type TEXT NOT NULL, -- user / admin / team
            sender_id   TEXT DEFAULT '',
            sender_name TEXT DEFAULT '',
            content     TEXT NOT NULL,
            created_at  INTEGER NOT NULL,
            read        INTEGER NOT NULL DEFAULT 0
        )`,
		`CREATE INDEX IF NOT EXISTS idx_support_msgs_ticket ON support_messages(ticket_id)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return fmt.Errorf("数据库迁移失败: %w", err)
		}
	}

	s.mu.Lock()
	s.db = db
	s.dbPath = dbPath
	s.mu.Unlock()

	// 兼容旧库：补充 images / parent_id / 服务类字段列（若已存在则忽略错误）
	_, _ = db.Exec(`ALTER TABLE products ADD COLUMN images TEXT DEFAULT '[]'`)
	_, _ = db.Exec(`ALTER TABLE products ADD COLUMN attributes TEXT DEFAULT '[]'`)
	_, _ = db.Exec(`ALTER TABLE products ADD COLUMN service INTEGER DEFAULT 0`)
	_, _ = db.Exec(`ALTER TABLE products ADD COLUMN source_team TEXT DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE categories ADD COLUMN parent_id TEXT DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE categories ADD COLUMN is_service INTEGER DEFAULT 0`)
	// 团队指定客服成员（接收/回复团队服务会话）
	_, _ = db.Exec(`ALTER TABLE teams ADD COLUMN support_member_phone TEXT DEFAULT ''`)
	// 团队金库（服务订单 90% 分成收入；仅团长可提取/转账）
	_, _ = db.Exec(`ALTER TABLE teams ADD COLUMN treasury REAL DEFAULT 0`)
	// 用户：邀请人绑定（被邀请人记录邀请人 user_id）
	_, _ = db.Exec(`ALTER TABLE users ADD COLUMN invited_by TEXT DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE users ADD COLUMN invited_at INTEGER DEFAULT 0`)
	// 订单：微信支付交易号（真实支付回调写入）
	_, _ = db.Exec(`ALTER TABLE orders ADD COLUMN transaction_id TEXT DEFAULT ''`)
	// 订单：退款时间（无理由退款写入）
	_, _ = db.Exec(`ALTER TABLE orders ADD COLUMN refund_time INTEGER`)

	// 两级类目结构迁移：旧版一级类目数据不兼容，自动重建并写入新版种子
	if err := s.migrateCatalog(); err != nil {
		return fmt.Errorf("类目结构初始化失败: %w", err)
	}
	// 固定"服务"大类：写死，幂等补齐（服务类商品仅后台与团队可发布）
	if err := s.EnsureServiceCatalog(); err != nil {
		return fmt.Errorf("服务大类初始化失败: %w", err)
	}
	// 恢复持久化用户账号（重启后订单/地址/提现仍与账号关联）
	s.LoadUsers()
	return nil
}

// migrateCatalog 两级类目迁移：
// 旧版只有一级类目（无 parent_id），数据结构不兼容，需重建 categories/products 并写入两级种子。
// 通过 sys_info.catalog_version 标记版本，仅重建一次；已是新版本时仅在空库时写入种子。
func (s *Store) migrateCatalog() error {
	if s.db == nil {
		return nil
	}
	var v string
	err := s.db.QueryRow(`SELECT value FROM sys_info WHERE key = 'catalog_version'`).Scan(&v)
	if err == sql.ErrNoRows || v != "2" {
		_, _ = s.db.Exec(`DELETE FROM products`)
		_, _ = s.db.Exec(`DELETE FROM categories`)
		if err := s.seedCatalog(); err != nil {
			return err
		}
		_, err = s.db.Exec(
			`INSERT INTO sys_info (key, value) VALUES ('catalog_version', '2')
             ON CONFLICT(key) DO UPDATE SET value = excluded.value`)
		return err
	}
	if err := s.seedCatalog(); err != nil {
		return err
	}
	// 升级补全：补齐服务类演示商品与种子商品的内置属性（幂等）
	return s.enrichSeedProducts()
}

// DBStatus 数据库连接状态（供工作台展示验证）
func (s *Store) DBStatus() map[string]interface{} {
	s.mu.RLock()
	db := s.db
	path := s.dbPath
	s.mu.RUnlock()

	status := map[string]interface{}{
		"connected":    false,
		"driver":       "sqlite",
		"path":         path,
		"tables":       []string{},
		"productEdits": 0,
	}
	if db == nil {
		return status
	}
	if err := db.Ping(); err != nil {
		return status
	}

	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' ORDER BY name`)
	if err == nil {
		tables := []string{}
		for rows.Next() {
			var name string
			if rows.Scan(&name) == nil {
				tables = append(tables, name)
			}
		}
		rows.Close()
		status["tables"] = tables
	}

	var cnt int
	if err := db.QueryRow(`SELECT COUNT(*) FROM product_edits`).Scan(&cnt); err == nil {
		status["productEdits"] = cnt
	}
	status["connected"] = true
	return status
}

// LogProductEdit 写入商品修改审计
func (s *Store) LogProductEdit(e ProductEdit) error {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return fmt.Errorf("数据库未连接")
	}
	if e.ID == "" {
		e.ID = randID("e")
	}
	if e.CreatedAt == 0 {
		e.CreatedAt = time.Now().UnixMilli()
	}
	_, err := db.Exec(
		`INSERT INTO product_edits (id, product_id, field, old_value, new_value, operator, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.ProductID, e.Field, e.OldValue, e.NewValue, e.Operator, e.CreatedAt,
	)
	return err
}

// ListProductEdits 读取最近商品修改审计（按时间倒序）
func (s *Store) ListProductEdits(limit int) ([]ProductEdit, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := db.Query(
		`SELECT id, product_id, field, old_value, new_value, operator, created_at
         FROM product_edits ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []ProductEdit{}
	for rows.Next() {
		var e ProductEdit
		if rows.Scan(&e.ID, &e.ProductID, &e.Field, &e.OldValue, &e.NewValue, &e.Operator, &e.CreatedAt) != nil {
			continue
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
