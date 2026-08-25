// catalog.go
// 分类与商品的数据库操作（SQLite 持久化，可动态管理）
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// ---------- JSON 辅助 ----------

func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func fromJSON(s string, out interface{}) {
	_ = json.Unmarshal([]byte(s), out)
}

// ---------- 种子数据（首次启动写入数据库）----------
// 两级类目：ParentID 为空 = 一级类目；非空 = 归属该一级类目的二级类目

var seedCategories = []Category{
	// 一级类目
	{ID: "monitor", Name: "智能监控", Sort: 0},
	{ID: "alarm", Name: "燃气烟雾", Sort: 1},
	{ID: "help", Name: "紧急求助", Sort: 2},
	{ID: "care", Name: "老人看护", Sort: 3},
	{ID: "lock", Name: "居家防护", Sort: 4},
	{ID: "service", Name: "服务", Sort: 5, IsService: true},
	// 二级类目
	{ID: "monitor_camera", Name: "智能摄像头", ParentID: "monitor", Sort: 0},
	{ID: "monitor_doorbell", Name: "可视门铃", ParentID: "monitor", Sort: 1},
	{ID: "alarm_smoke", Name: "烟雾报警器", ParentID: "alarm", Sort: 0},
	{ID: "alarm_gas", Name: "燃气报警器", ParentID: "alarm", Sort: 1},
	{ID: "help_sos", Name: "紧急按钮", ParentID: "help", Sort: 0},
	{ID: "help_fall", Name: "跌倒监测", ParentID: "help", Sort: 1},
	{ID: "care_watch", Name: "定位手表", ParentID: "care", Sort: 0},
	{ID: "care_sleep", Name: "睡眠监测", ParentID: "care", Sort: 1},
	{ID: "lock_door", Name: "智能门锁", ParentID: "lock", Sort: 0},
	{ID: "lock_infrared", Name: "红外感应", ParentID: "lock", Sort: 1},
	{ID: "lock_water", Name: "水浸传感器", ParentID: "lock", Sort: 2},
	// 服务类二级类目（服务大类下，后台/团队发布服务商品）
	{ID: "service_install", Name: "安装服务", ParentID: "service", Sort: 0},
	{ID: "service_care", Name: "看护服务", ParentID: "service", Sort: 1},
	{ID: "service_custom", Name: "定制服务", ParentID: "service", Sort: 2},
}

// ServiceCategoryID 固定的服务大类 ID（写死：仅后台与团队可发布服务类商品）
const ServiceCategoryID = "service"

// ServiceCategoryIDs 服务大类下所有类目 ID（含一级与二级，用于判定商品是否服务类）
var ServiceCategoryIDs = []string{ServiceCategoryID, "service_install", "service_care", "service_custom"}

// IsServiceCategory 判断类目是否属于服务大类（自身或其上级）
func IsServiceCategory(c *Category) bool {
	if c == nil {
		return false
	}
	if c.IsService {
		return true
	}
	if c.ParentID == ServiceCategoryID {
		return true
	}
	return false
}

// EnsureServiceCatalog 幂等补齐"服务"大类与其二级类目（用于旧库升级）
func (s *Store) EnsureServiceCatalog() error {
	if s.db == nil {
		return nil
	}
	existing, err := s.GetCategory(ServiceCategoryID)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}
	for _, c := range seedCategories {
		if c.ParentID == ServiceCategoryID || c.ID == ServiceCategoryID {
			if err := s.SaveCategory(c); err != nil {
				return err
			}
		}
	}
	return nil
}

var seedProducts = []Product{
	{
		ID: "p001", Name: "高清智能摄像头", Desc: "1080P 超清夜视 · 双向语音 · 移动侦测",
		Price: 299, OriginalPrice: 399, Emoji: "📹",
		Colors: []string{"#3B82F6", "#1E40AF"},
		Attributes: []ProductAttribute{
			{Name: "版本", Values: []string{"标准版", "云台版"}},
		},
		Sales:  1024, Category: "monitor_camera", Tags: []string{"热卖", "包邮"},
		Detail: []string{
			"1080P 全高清画质，红外夜视清晰不模糊",
			"支持双向语音通话，随时和家里交流",
			"智能移动侦测，异常情况实时推送报警",
			"云存储 + 本地存储双保障，回看更安心",
		},
	},
	{
		ID: "p002", Name: "智能可视门铃", Desc: "门前实时可视 · 变声应答 · 访客抓拍",
		Price: 599, OriginalPrice: 699, Emoji: "📷",
		Colors: []string{"#22D3EE", "#0E7490"},
		Sales:  386, Category: "monitor_doorbell", Tags: []string{"新品"},
		Detail: []string{
			"门前动态实时可视，白天黑夜都清晰",
			"变声应答功能，独居更安全",
			"访客自动抓拍，不在家也能掌握动态",
			"超长续航，支持快速充电",
		},
	},
	{
		ID: "p003", Name: "独立式烟雾报警器", Desc: "烟雾实时探测 · 高分贝报警 · 静音键",
		Price: 89, OriginalPrice: 129, Emoji: "🔥",
		Colors: []string{"#F97316", "#C2410C"},
		Sales:  2158, Category: "alarm_smoke", Tags: []string{"热卖"},
		Detail: []string{
			"高灵敏度烟雾传感器，实时探测火情",
			"85 分贝高分贝蜂鸣，全屋清晰可闻",
			"低电量提示，一键静音防误报",
			"3C 认证，使用更放心",
		},
	},
	{
		ID: "p004", Name: "燃气泄漏报警器", Desc: "甲烷丙烷检测 · 声光双报警 · 自动断电",
		Price: 129, OriginalPrice: 169, Emoji: "🔥",
		Colors: []string{"#EF4444", "#991B1B"},
		Sales:  968, Category: "alarm_gas", Tags: []string{},
		Detail: []string{
			"精准检测甲烷、丙烷等可燃气体",
			"浓度超标声光双报警，及时提醒",
			"可联动燃气阀门，自动关闭更安全",
			"大屏数显，浓度一目了然",
		},
	},
	{
		ID: "p005", Name: "一键紧急求助按钮", Desc: "一键呼叫 · 拉绳触发 · 老人首选",
		Price: 59, OriginalPrice: 79, Emoji: "🆘",
		Colors: []string{"#F43F5E", "#9F1239"},
		Sales:  1687, Category: "help_sos", Tags: []string{"热卖"},
		Detail: []string{
			"按键 + 拉绳双触发方式，紧急时刻一触即达",
			"关联家人手机号，按下立即电话 + 短信通知",
			"大按键大音量，老人轻松使用",
			"免布线安装，随处可贴",
		},
	},
	{
		ID: "p006", Name: "跌倒监测智能手环", Desc: "跌倒自动报警 · 心率血氧 · 防水长续航",
		Price: 499, OriginalPrice: 599, Emoji: "⌚",
		Colors: []string{"#8B5CF6", "#5B21B6"},
		Attributes: []ProductAttribute{
			{Name: "表带颜色", Values: []string{"曜石黑", "雾霾蓝", "樱花粉"}},
			{Name: "腕带尺寸", Values: []string{"S", "M", "L"}},
		},
		Sales:  745, Category: "help_fall", Tags: []string{"新品"},
		Detail: []string{
			"AI 跌倒算法，检测到跌倒自动向家人报警",
			"24 小时心率血氧监测，健康数据随时看",
			"IP68 防水，洗澡洗手不用摘",
			"大容量电池，一次充电用一周",
		},
	},
	{
		ID: "p007", Name: "老人GPS定位手表", Desc: "实时定位 · 电子围栏 · SOS 求救",
		Price: 399, OriginalPrice: 499, Emoji: "⌚",
		Colors: []string{"#06B6D4", "#155E75"},
		Sales:  1320, Category: "care_watch", Tags: []string{"热卖"},
		Detail: []string{
			"GPS + LBS 双重定位，位置实时可查",
			"电子围栏功能，走出安全区域自动提醒",
			"SOS 一键求救，紧急情况自动呼叫家人",
			"超长待机，免频繁充电",
		},
	},
	{
		ID: "p008", Name: "智能睡眠监测仪", Desc: "零接触监测 · 呼吸心率 · 起夜提醒",
		Price: 899, OriginalPrice: 1099, Emoji: "💤",
		Colors: []string{"#6366F1", "#3730A3"},
		Sales:  218, Category: "care_sleep", Tags: []string{},
		Detail: []string{
			"零接触式监测，无需穿戴更舒适",
			"睡眠期间呼吸、心率、翻身全面监测",
			"起夜自动检测并柔光提醒，防跌倒",
			"睡眠报告每日推送，健康更省心",
		},
	},
	{
		ID: "p009", Name: "智能指纹门锁", Desc: "指纹密码刷卡 · C 级锁芯 · 防撬报警",
		Price: 1299, OriginalPrice: 1599, Emoji: "🔒",
		Colors: []string{"#475569", "#1E293B"},
		Sales:  456, Category: "lock_door", Tags: []string{},
		Detail: []string{
			"指纹、密码、刷卡、钥匙多种开锁方式",
			"C 级安全锁芯，防技术开启",
			"防撬报警 + 远程通知，异常实时知晓",
			"虚位密码防偷窥，安全更贴心",
		},
	},
	{
		ID: "p010", Name: "红外人体感应报警器", Desc: "移动侦测 · 双向布防 · 语音告警",
		Price: 169, OriginalPrice: 199, Emoji: "🚨",
		Colors: []string{"#F59E0B", "#B45309"},
		Sales:  612, Category: "lock_infrared", Tags: []string{"新品"},
		Detail: []string{
			"红外人体感应，8 米探测范围",
			"一键布防/撤防，白天自动休眠省电",
			"有人闯入即响警报并推送手机",
			"语音告警，震慑非法闯入",
		},
	},
	{
		ID: "p011", Name: "水浸传感器", Desc: "漏水检测 · 声光报警 · 联动关阀",
		Price: 99, OriginalPrice: 129, Emoji: "💧",
		Colors: []string{"#0EA5E9", "#075985"},
		Sales:  534, Category: "lock_water", Tags: []string{},
		Detail: []string{
			"灵敏检测漏水，渗水、溢水第一时间发现",
			"本地声光报警，远程同步提醒",
			"可联动智能阀门，自动切断水源",
			"贴片式设计，厨房卫生间均可放置",
		},
	},
	{
		ID: "p012", Name: "安全设备上门安装调试", Desc: "专业工程师上门安装调试各类安全监护设备",
		Price: 99, OriginalPrice: 149, Emoji: "🔧",
		Colors: []string{"#0EA5E9", "#075985"},
		Attributes: []ProductAttribute{
			{Name: "安装数量", Values: []string{"1 台", "2-3 台", "4 台及以上"}},
			{Name: "服务时间", Values: []string{"工作日", "周末"}},
		},
		Sales:  320, Category: "service_install", Tags: []string{"官方服务"},
		Service:    true,
		SourceTeam: "官方服务",
		Detail: []string{
			"专业工程师上门，覆盖全城",
			"设备安装 + 调试 + 使用指导一站式服务",
			"服务完成 30 天内免费复检",
			"来源：官方服务团队",
		},
	},
}

// ---------- 分类操作 ----------

// ListCategories 分类列表（按 sort 排序）
func (s *Store) ListCategories() ([]Category, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	rows, err := db.Query(`SELECT id, name, parent_id, sort, is_service FROM categories ORDER BY sort, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Category{}
	for rows.Next() {
		var c Category
		var isService int
		if rows.Scan(&c.ID, &c.Name, &c.ParentID, &c.Sort, &isService) != nil {
			continue
		}
		c.IsService = isService == 1
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetCategory 获取分类
func (s *Store) GetCategory(id string) (*Category, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	var c Category
	var isService int
	err := db.QueryRow(`SELECT id, name, parent_id, sort, is_service FROM categories WHERE id = ?`, id).
		Scan(&c.ID, &c.Name, &c.ParentID, &c.Sort, &isService)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.IsService = isService == 1
	return &c, nil
}

// SaveCategory 新增或更新分类
func (s *Store) SaveCategory(c Category) error {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return fmt.Errorf("数据库未连接")
	}
	isService := 0
	if c.IsService {
		isService = 1
	}
	_, err := db.Exec(
		`INSERT INTO categories (id, name, parent_id, sort, is_service) VALUES (?, ?, ?, ?, ?)
         ON CONFLICT(id) DO UPDATE SET name=excluded.name, parent_id=excluded.parent_id, sort=excluded.sort, is_service=excluded.is_service`,
		c.ID, c.Name, c.ParentID, c.Sort, isService)
	return err
}

// DeleteCategory 删除分类（一级类目时级联删除其下二级类目；商品保留类目字段）
func (s *Store) DeleteCategory(id string) error {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return fmt.Errorf("数据库未连接")
	}
	_, err := db.Exec(`DELETE FROM categories WHERE id = ? OR parent_id = ?`, id, id)
	return err
}

// ---------- 商品操作 ----------

func scanProduct(row *sql.Row) (*Product, error) {
	var p Product
	var colors, images, tags, detail, attributes string
	var service int
	err := row.Scan(&p.ID, &p.Name, &p.Desc, &p.Price, &p.OriginalPrice,
		&p.Emoji, &colors, &images, &p.Sales, &p.Category, &tags, &detail,
		&attributes, &service, &p.SourceTeam, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	fromJSON(colors, &p.Colors)
	fromJSON(images, &p.Images)
	fromJSON(tags, &p.Tags)
	fromJSON(detail, &p.Detail)
	fromJSON(attributes, &p.Attributes)
	p.Service = service == 1
	return &p, nil
}

// scanProductRow 从 *sql.Rows 扫描一行商品
func scanProductRow(rows *sql.Rows, p *Product) bool {
	var colors, images, tags, detail, attributes string
	var service int
	if rows.Scan(&p.ID, &p.Name, &p.Desc, &p.Price, &p.OriginalPrice,
		&p.Emoji, &colors, &images, &p.Sales, &p.Category, &tags, &detail,
		&attributes, &service, &p.SourceTeam, &p.UpdatedAt) != nil {
		return false
	}
	fromJSON(colors, &p.Colors)
	fromJSON(images, &p.Images)
	fromJSON(tags, &p.Tags)
	fromJSON(detail, &p.Detail)
	fromJSON(attributes, &p.Attributes)
	p.Service = service == 1
	return true
}

func (s *Store) insertProduct(p Product) error {
	images := toJSON(p.Images)
	if p.Images == nil {
		images = "[]"
	}
	service := 0
	if p.Service {
		service = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO products (id, name, desc, price, original_price, emoji, colors, images, sales, category, tags, detail, attributes, service, source_team, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Desc, p.Price, p.OriginalPrice, p.Emoji,
		toJSON(p.Colors), images, p.Sales, p.Category, toJSON(p.Tags), toJSON(p.Detail),
		toJSON(p.Attributes), service, p.SourceTeam, timeNow(),
	)
	return err
}

// ListProducts 商品列表（category / keyword 筛选）
func (s *Store) ListProducts(category, keyword string) ([]Product, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	kw := strings.ToLower(strings.TrimSpace(keyword))
	query := `SELECT id, name, desc, price, original_price, emoji, colors, images, sales, category, tags, detail, attributes, service, source_team, updated_at FROM products`
	args := []interface{}{}
	if category != "" {
		query += ` WHERE category = ?`
		args = append(args, category)
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Product{}
	for rows.Next() {
		var p Product
		if !scanProductRow(rows, &p) {
			continue
		}
		if kw != "" {
			if !strings.Contains(strings.ToLower(p.Name), kw) &&
				!strings.Contains(strings.ToLower(p.Desc), kw) {
				continue
			}
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetProduct 获取商品
func (s *Store) GetProduct(id string) (*Product, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	row := db.QueryRow(
		`SELECT id, name, desc, price, original_price, emoji, colors, images, sales, category, tags, detail, attributes, service, source_team, updated_at FROM products WHERE id = ?`, id)
	return scanProduct(row)
}

// UpsertProduct 新增或更新商品（Excel 导入用）
func (s *Store) UpsertProduct(p Product) error {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return fmt.Errorf("数据库未连接")
	}
	images := toJSON(p.Images)
	if p.Images == nil {
		images = "[]"
	}
	service := 0
	if p.Service {
		service = 1
	}
	_, err := db.Exec(
		`INSERT INTO products (id, name, desc, price, original_price, emoji, colors, images, sales, category, tags, detail, attributes, service, source_team, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
         ON CONFLICT(id) DO UPDATE SET
            name=excluded.name, desc=excluded.desc, price=excluded.price,
            original_price=excluded.original_price, emoji=excluded.emoji,
            colors=excluded.colors, images=excluded.images, sales=excluded.sales,
            category=excluded.category, tags=excluded.tags, detail=excluded.detail,
            attributes=excluded.attributes, service=excluded.service, source_team=excluded.source_team,
            updated_at=excluded.updated_at`,
		p.ID, p.Name, p.Desc, p.Price, p.OriginalPrice, p.Emoji,
		toJSON(p.Colors), images, p.Sales, p.Category, toJSON(p.Tags), toJSON(p.Detail),
		toJSON(p.Attributes), service, p.SourceTeam, timeNow(),
	)
	return err
}

// UpdateProduct 更新商品字段
func (s *Store) UpdateProduct(id string, price, originalPrice *float64, sales *int) (*Product, error) {
	p, err := s.GetProduct(id)
	if err != nil || p == nil {
		return p, err
	}
	if price != nil {
		p.Price = *price
	}
	if originalPrice != nil {
		p.OriginalPrice = *originalPrice
	}
	if sales != nil {
		p.Sales = *sales
	}
	if err := s.UpsertProduct(*p); err != nil {
		return nil, err
	}
	return p, nil
}

// DeleteProduct 删除商品
func (s *Store) DeleteProduct(id string) error {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return fmt.Errorf("数据库未连接")
	}
	_, err := db.Exec(`DELETE FROM products WHERE id = ?`, id)
	return err
}

// CountCategories 分类数量
func (s *Store) CountCategories() int {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return 0
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM categories`).Scan(&n)
	return n
}

// CountProducts 商品数量
func (s *Store) CountProducts() int {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return 0
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM products`).Scan(&n)
	return n
}

// seedCatalog 首次启动写入内置分类与商品
func (s *Store) seedCatalog() error {
	if s.db == nil {
		return nil
	}
	var cnt int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM categories`).Scan(&cnt); err == nil && cnt == 0 {
		for _, c := range seedCategories {
			if err := s.SaveCategory(c); err != nil {
				return err
			}
		}
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM products`).Scan(&cnt); err == nil && cnt == 0 {
		for _, p := range seedProducts {
			if err := s.insertProduct(p); err != nil {
				return err
			}
		}
	}
	return nil
}

// enrichSeedProducts 幂等补全种子商品的新增字段（升级用）：
// - 缺失的种子商品（如服务类演示商品 p012）自动补入
// - 已存在的种子商品若属性为空，补齐种子定义的内置属性/服务标记
func (s *Store) enrichSeedProducts() error {
	if s.db == nil {
		return nil
	}
	for _, sp := range seedProducts {
		existing, err := s.GetProduct(sp.ID)
		if err != nil {
			return err
		}
		if existing == nil {
			if err := s.insertProduct(sp); err != nil {
				return err
			}
			continue
		}
		// 属性为空且种子定义了属性 -> 补齐（保留名称/价格等已有数据）
		if (existing.Attributes == nil || len(existing.Attributes) == 0) && len(sp.Attributes) > 0 {
			existing.Attributes = sp.Attributes
			if err := s.UpsertProduct(*existing); err != nil {
				return err
			}
		}
	}
	return nil
}
