// admin.go
// 管理端接口（工作台使用）
// 管理员登录后使用独立 admin token 访问 /api/v1/admin/*
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"socialserver/internal/store"
)

type adminLoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// POST /api/v1/admin/login 管理端账号登录（admin / staff）
func (a *API) adminLogin(w http.ResponseWriter, r *http.Request) {
	var req adminLoginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if strings.TrimSpace(req.Username) == "" || req.Password == "" {
		fail(w, http.StatusBadRequest, "用户名和密码不能为空")
		return
	}
	if tooLong(req.Username, maxUsername) || tooLong(req.Password, maxPassword) {
		fail(w, http.StatusBadRequest, "用户名或密码长度不正确")
		return
	}
	acc, err := a.store.GetAdminAccountByUsername(strings.TrimSpace(req.Username))
	if err != nil {
		fail(w, http.StatusInternalServerError, "账号查询失败")
		return
	}
	if acc == nil || !store.CheckPassword(acc.PasswordHash, req.Password) {
		fail(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	if acc.Status != 1 {
		fail(w, http.StatusForbidden, "账号已被停用，请联系超级管理员")
		return
	}
	token := a.store.CreateAdminToken(acc.ID, acc.Role)
	ok(w, map[string]interface{}{
		"token":    token,
		"role":     acc.Role,
		"username": acc.Username,
	})
}

// ---------- 管理端鉴权 ----------

type adminCtxKey int

const adminKey adminCtxKey = 0

// accountFrom 从请求上下文获取当前管理端账号
func accountFrom(r *http.Request) *store.AdminAccount {
	a, _ := r.Context().Value(adminKey).(*store.AdminAccount)
	return a
}

// adminAuth 管理端鉴权中间件（admin 与 staff 均可通过，账号须有效且启用）
func (a *API) adminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			fail(w, http.StatusUnauthorized, "未登录")
			return
		}
		accountID, role, ok := a.store.VerifyAdminToken(token)
		if !ok {
			fail(w, http.StatusUnauthorized, "登录已失效，请重新登录")
			return
		}
		acc, err := a.store.GetAdminAccountByID(accountID)
		if err != nil || acc == nil {
			fail(w, http.StatusUnauthorized, "账号不存在")
			return
		}
		if acc.Status != 1 {
			fail(w, http.StatusForbidden, "账号已被停用")
			return
		}
		acc.Role = role
		ctx := context.WithValue(r.Context(), adminKey, acc)
		next(w, r.WithContext(ctx))
	}
}

// superAdminAuth 仅超级管理员可访问（账号管理、权限修改等）
// 员工账号无此权限 → 无法修改自己账号权限
func (a *API) superAdminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc := accountFrom(r)
		if acc == nil || acc.Role != "admin" {
			fail(w, http.StatusForbidden, "无权限：仅超级管理员可执行该操作")
			return
		}
		next(w, r)
	}
}

// GET /api/v1/admin/stats 数据统计
func (a *API) adminStats(w http.ResponseWriter, r *http.Request) {
	ok(w, a.store.Stats())
}

// GET /api/v1/admin/users 用户列表
func (a *API) adminUsers(w http.ResponseWriter, r *http.Request) {
	ok(w, a.store.ListUsers())
}

// GET /api/v1/admin/devices 设备列表（含设备类型信息）
func (a *API) adminDevices(w http.ResponseWriter, r *http.Request) {
	devices := a.store.ListAllDevices()
	out := make([]map[string]interface{}, 0, len(devices))
	for _, d := range devices {
		item := map[string]interface{}{
			"id":            d.ID,
			"userId":        d.UserID,
			"name":          d.Name,
			"type":          d.Type,
			"typeName":      deviceTypes[d.Type],
			"sn":            d.SN,
			"status":        d.Status,
			"battery":       d.Battery,
			"notifyEnabled": d.NotifyEnabled,
			"lastActive":    d.LastActive,
			"createTime":    d.CreateTime,
		}
		out = append(out, item)
	}
	ok(w, out)
}

// GET /api/v1/admin/messages 消息列表
func (a *API) adminMessages(w http.ResponseWriter, r *http.Request) {
	ok(w, a.store.ListAllMessages())
}

// GET /api/v1/admin/products 商品列表
func (a *API) adminProducts(w http.ResponseWriter, r *http.Request) {
	products, err := a.store.ListProducts("", "")
	if err != nil {
		fail(w, http.StatusInternalServerError, "商品数据读取失败: "+err.Error())
		return
	}
	ok(w, products)
}

type createProductReq struct {
	ID            string                   `json:"id"`
	Name          string                   `json:"name"`
	Desc          string                   `json:"desc"`
	Price         float64                  `json:"price"`
	OriginalPrice float64                  `json:"originalPrice"`
	Emoji         string                   `json:"emoji"`
	Colors        []string                 `json:"colors"`
	Images        []string                 `json:"images"` // 商品图片 URL 数组
	Sales         int                      `json:"sales"`
	Category      string                   `json:"category"`
	Tags          []string                 `json:"tags"`
	Detail        []string                 `json:"detail"`
	Attributes    []store.ProductAttribute `json:"attributes"`    // 内置属性（创建者定义）
	SourceTeam    string                   `json:"sourceTeam"`    // 服务来源（服务类商品）
}

// POST /api/v1/admin/products 新增商品
func (a *API) adminCreateProduct(w http.ResponseWriter, r *http.Request) {
	var req createProductReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || req.Price <= 0 {
		fail(w, http.StatusBadRequest, "商品名称与价格必填")
		return
	}
	if tooLong(req.Name, maxProductName) {
		fail(w, http.StatusBadRequest, "商品名称最长 60 个字符")
		return
	}
	if req.Price > 999999 || req.Price <= 0 {
		fail(w, http.StatusBadRequest, "商品价格须在 0 ~ 999999 元之间")
		return
	}
	if req.OriginalPrice < 0 || req.OriginalPrice > 999999 {
		fail(w, http.StatusBadRequest, "商品原价超出合理范围（0 ~ 999999）")
		return
	}
	if req.Sales < 0 || req.Sales > 100000000 {
		fail(w, http.StatusBadRequest, "销量必须为 0 ~ 1 亿之间的非负整数")
		return
	}
	if tooLong(req.Desc, maxDesc) {
		fail(w, http.StatusBadRequest, "商品描述最长 200 个字符")
		return
	}
	if tooLong(req.Emoji, 8) {
		fail(w, http.StatusBadRequest, "商品图标过长")
		return
	}
	if len(req.Tags) > 10 {
		fail(w, http.StatusBadRequest, "标签最多 10 个")
		return
	}
	for _, t := range req.Tags {
		if tooLong(t, 20) {
			fail(w, http.StatusBadRequest, "单个标签最长 20 个字符")
			return
		}
	}
	if len(req.Detail) > 50 {
		fail(w, http.StatusBadRequest, "图文详情最多 50 条")
		return
	}
	for _, d := range req.Detail {
		if tooLong(d, 100) {
			fail(w, http.StatusBadRequest, "图文详情单条最长 100 个字符")
			return
		}
	}
	if len(req.Attributes) > 20 {
		fail(w, http.StatusBadRequest, "属性最多 20 项")
		return
	}
	for _, at := range req.Attributes {
		if tooLong(at.Name, 20) {
			fail(w, http.StatusBadRequest, "属性名称最长 20 个字符")
			return
		}
		if len(at.Values) > 20 {
			fail(w, http.StatusBadRequest, "单个属性取值最多 20 个")
			return
		}
		for _, v := range at.Values {
			if tooLong(v, 50) {
				fail(w, http.StatusBadRequest, "单个属性值最长 50 个字符")
				return
			}
		}
	}
	if req.ID == "" {
		req.ID = "p" + fmt.Sprintf("%03d", a.store.CountProducts()+1)
	}
	if req.OriginalPrice < req.Price {
		req.OriginalPrice = req.Price
	}
	// 服务类商品判定：类目属于"服务"大类时强制标记服务属性
	service := false
	sourceTeam := req.SourceTeam
	if cat, err := a.store.GetCategory(req.Category); err == nil && cat != nil {
		if store.IsServiceCategory(cat) {
			service = true
			if sourceTeam == "" {
				sourceTeam = "官方服务"
			}
		}
	}
	p := store.Product{
		ID: req.ID, Name: req.Name, Desc: req.Desc,
		Price: req.Price, OriginalPrice: req.OriginalPrice,
		Emoji: req.Emoji, Colors: req.Colors, Images: req.Images, Sales: req.Sales,
		Category: req.Category, Tags: req.Tags, Detail: req.Detail,
		Attributes: req.Attributes, Service: service, SourceTeam: sourceTeam,
	}
	if err := a.store.UpsertProduct(p); err != nil {
		fail(w, http.StatusInternalServerError, "商品保存失败: "+err.Error())
		return
	}
	ok(w, p)
}

// DELETE /api/v1/admin/products/{id} 删除商品
func (a *API) adminDeleteProduct(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := a.store.GetProduct(id)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if p == nil {
		fail(w, http.StatusNotFound, "商品不存在")
		return
	}
	if err := a.store.DeleteProduct(id); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, map[string]string{"id": id})
}

// batchProductReq 批量导入请求（前端解析 Excel 后提交 JSON）
type batchProductReq struct {
	Products []store.Product `json:"products"`
}

// POST /api/v1/admin/products/batch 批量新增/更新商品
//
// Excel 由前端（工作台）解析为 JSON 后提交，服务器负责最终校验 + 审计落库
func (a *API) adminBatchProducts(w http.ResponseWriter, r *http.Request) {
	var req batchProductReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if len(req.Products) == 0 {
		fail(w, http.StatusBadRequest, "无商品数据")
		return
	}

	operator := "admin"
	imported, updated, failed := 0, 0, 0
	for _, p := range req.Products {
		// 强校验
		if p.Name == "" || tooLong(p.Name, maxProductName) || p.Price <= 0 || p.Price > 999999 || p.Sales < 0 || p.Sales > 100000000 {
			failed++
			continue
		}
		if tooLong(p.Desc, maxDesc) {
			failed++
			continue
		}
		if p.OriginalPrice < p.Price {
			p.OriginalPrice = p.Price
		}
		if p.ID == "" {
			p.ID = fmt.Sprintf("p%03d", time.Now().UnixNano()%100000)
		}
		existing, err := a.store.GetProduct(p.ID)
		if err != nil {
			failed++
			continue
		}
		if err := a.store.UpsertProduct(p); err != nil {
			failed++
			continue
		}
		if existing != nil {
			updated++
			// 审计：记录变更字段
			if existing.Price != p.Price {
				_ = a.store.LogProductEdit(store.ProductEdit{
					ProductID: p.ID, Field: "price",
					OldValue: fmt.Sprintf("%v", existing.Price), NewValue: fmt.Sprintf("%v", p.Price),
					Operator: operator,
				})
			}
			if existing.Sales != p.Sales {
				_ = a.store.LogProductEdit(store.ProductEdit{
					ProductID: p.ID, Field: "sales",
					OldValue: fmt.Sprintf("%v", existing.Sales), NewValue: fmt.Sprintf("%v", p.Sales),
					Operator: operator,
				})
			}
		} else {
			imported++
		}
	}

	ok(w, map[string]interface{}{
		"total":    len(req.Products),
		"imported": imported,
		"updated":  updated,
		"failed":   failed,
	})
}

// ---------- 类目管理 ----------

// GET /api/v1/admin/categories 分类列表
func (a *API) adminListCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := a.store.ListCategories()
	if err != nil {
		fail(w, http.StatusInternalServerError, "分类数据读取失败: "+err.Error())
		return
	}
	ok(w, cats)
}

type categoryReq struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ParentID  string `json:"parentId"` // 空 = 一级类目；非空 = 所属一级类目
	Sort      int    `json:"sort"`
	IsService bool   `json:"isService"` // 服务大类（写死：仅后台可标记，服务类商品发布来源）
}

// POST /api/v1/admin/categories 新增分类（一级或二级类目）
func (a *API) adminCreateCategory(w http.ResponseWriter, r *http.Request) {
	var req categoryReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if req.Name == "" {
		fail(w, http.StatusBadRequest, "分类名称必填")
		return
	}
	if tooLong(req.Name, 20) {
		fail(w, http.StatusBadRequest, "分类名称最长 20 个字符")
		return
	}
	if req.Sort < 0 || req.Sort > 999 {
		fail(w, http.StatusBadRequest, "排序值不合理（0 ~ 999）")
		return
	}
	if req.ParentID != "" {
		parent, err := a.store.GetCategory(req.ParentID)
		if err != nil || parent == nil {
			fail(w, http.StatusBadRequest, "所属一级类目不存在")
			return
		}
		if parent.ParentID != "" {
			fail(w, http.StatusBadRequest, "二级类目的上级只能是一级类目")
			return
		}
	}
	if req.IsService && req.ParentID != "" {
		fail(w, http.StatusBadRequest, "只有一级类目可标记为服务大类")
		return
	}
	if req.ID == "" {
		req.ID = "c" + fmt.Sprintf("%03d", time.Now().UnixNano()%1000)
	}
	c := store.Category{ID: req.ID, Name: req.Name, ParentID: req.ParentID, Sort: req.Sort, IsService: req.IsService}
	if err := a.store.SaveCategory(c); err != nil {
		fail(w, http.StatusInternalServerError, "分类保存失败: "+err.Error())
		return
	}
	ok(w, c)
}

// PUT /api/v1/admin/categories/{id} 更新分类
func (a *API) adminUpdateCategory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req categoryReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if req.Name == "" {
		fail(w, http.StatusBadRequest, "分类名称不能为空")
		return
	}
	if req.ParentID != "" {
		parent, err := a.store.GetCategory(req.ParentID)
		if err != nil || parent == nil {
			fail(w, http.StatusBadRequest, "所属一级类目不存在")
			return
		}
		if parent.ParentID != "" {
			fail(w, http.StatusBadRequest, "二级类目的上级只能是一级类目")
			return
		}
	}
	// 固定的"服务"大类：不允许取消服务标记（写死）
	isService := req.IsService
	if id == store.ServiceCategoryID {
		isService = true
	}
	c := store.Category{ID: id, Name: req.Name, ParentID: req.ParentID, Sort: req.Sort, IsService: isService}
	if err := a.store.SaveCategory(c); err != nil {
		fail(w, http.StatusInternalServerError, "分类保存失败: "+err.Error())
		return
	}
	ok(w, c)
}

// DELETE /api/v1/admin/categories/{id} 删除分类
func (a *API) adminDeleteCategory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == store.ServiceCategoryID {
		fail(w, http.StatusBadRequest, "「服务」为固定大类，不可删除")
		return
	}
	if err := a.store.DeleteCategory(id); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, map[string]string{"id": id})
}

type updateProductReq struct {
	Price         *float64 `json:"price"`
	OriginalPrice *float64 `json:"originalPrice"`
	Sales         *int     `json:"sales"`
}

// PUT /api/v1/admin/products/{id} 更新商品（安全增强：强校验 + 审计日志）
func (a *API) adminUpdateProduct(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req updateProductReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	// 强校验
	if req.Price != nil && (*req.Price <= 0 || *req.Price > 999999) {
		fail(w, http.StatusBadRequest, "价格必须在 0 ~ 999999 之间")
		return
	}
	if req.OriginalPrice != nil && (*req.OriginalPrice < 0 || *req.OriginalPrice > 999999) {
		fail(w, http.StatusBadRequest, "原价必须在 0 ~ 999999 之间")
		return
	}
	if req.Sales != nil && (*req.Sales < 0 || *req.Sales > 100000000) {
		fail(w, http.StatusBadRequest, "销量必须为 0 ~ 1 亿之间的非负整数")
		return
	}
	old, err := a.store.GetProduct(id)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if old == nil {
		fail(w, http.StatusNotFound, "商品不存在")
		return
	}
	// 快照旧值（GetProduct 返回指针，UpdateProduct 会原地修改，必须提前拷贝）
	oldPrice := old.Price
	oldOriginalPrice := old.OriginalPrice
	oldSales := old.Sales

	// 原价不得低于现价
	newPrice := oldPrice
	if req.Price != nil {
		newPrice = *req.Price
	}
	if req.OriginalPrice != nil && *req.OriginalPrice < newPrice {
		fail(w, http.StatusBadRequest, "原价不能低于现价")
		return
	}

	p, err := a.store.UpdateProduct(id, req.Price, req.OriginalPrice, req.Sales)
	if err != nil {
		fail(w, http.StatusInternalServerError, "商品保存失败: "+err.Error())
		return
	}

	// 审计：记录每个实际变更的字段
	edits := []map[string]string{}
	operator := "admin"
	if u := userFrom(r); u != nil {
		operator = u.Phone
	}
	record := func(field, oldV, newV string) {
		if oldV == newV {
			return
		}
		a.store.LogProductEdit(store.ProductEdit{
			ProductID: id,
			Field:     field,
			OldValue:  oldV,
			NewValue:  newV,
			Operator:  operator,
		})
		edits = append(edits, map[string]string{
			"field": field, "oldValue": oldV, "newValue": newV,
		})
	}
	if req.Price != nil {
		record("price", fmt.Sprintf("%v", oldPrice), fmt.Sprintf("%v", *req.Price))
	}
	if req.OriginalPrice != nil {
		record("originalPrice", fmt.Sprintf("%v", oldOriginalPrice), fmt.Sprintf("%v", *req.OriginalPrice))
	}
	if req.Sales != nil {
		record("sales", fmt.Sprintf("%v", oldSales), fmt.Sprintf("%v", *req.Sales))
	}

	ok(w, map[string]interface{}{
		"product": p,
		"audit":   edits,
	})
}

// POST /api/v1/admin/products/upload 上传商品图片（multipart，字段 file）
//
//	保存到 uploads/ 目录，返回可访问的相对 URL，如 /uploads/img_xxx.jpg
func (a *API) adminUploadImage(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10MB
		fail(w, http.StatusBadRequest, "上传文件过大或格式错误")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		fail(w, http.StatusBadRequest, "请选择图片文件（字段名 file）")
		return
	}
	defer file.Close()

	// 校验扩展名
	ext := strings.ToLower(filepath.Ext(header.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
	default:
		fail(w, http.StatusBadRequest, "仅支持 jpg / png / gif / webp 图片")
		return
	}

	if err := os.MkdirAll(a.uploadDir, 0o755); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	name := fmt.Sprintf("img_%d%s", time.Now().UnixNano(), ext)
	dst, err := os.Create(filepath.Join(a.uploadDir, name))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	ok(w, map[string]string{
		"url":  "/uploads/" + name,
		"name": name,
	})
}

// GET /api/v1/admin/db-status 数据库连接状态（工作台验证）
func (a *API) adminDBStatus(w http.ResponseWriter, r *http.Request) {
	status := a.store.DBStatus()
	// 附带最近审计记录
	if edits, err := a.store.ListProductEdits(10); err == nil {
		status["recentEdits"] = edits
	} else {
		status["recentEdits"] = []store.ProductEdit{}
	}
	ok(w, status)
}

// ---------- 账号管理（仅超级管理员 admin） ----------

// GET /api/v1/admin/accounts 账号列表
func (a *API) adminListAccounts(w http.ResponseWriter, r *http.Request) {
	list, err := a.store.ListAdminAccounts()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, list)
}

type createAccountReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"` // 可选，默认 staff
}

// POST /api/v1/admin/accounts 创建员工账号
func (a *API) adminCreateAccount(w http.ResponseWriter, r *http.Request) {
	var req createAccountReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	username := strings.TrimSpace(req.Username)
	if len(username) < 3 {
		fail(w, http.StatusBadRequest, "用户名至少 3 个字符")
		return
	}
	if tooLong(username, maxUsername) {
		fail(w, http.StatusBadRequest, "用户名最长 32 个字符")
		return
	}
	if len(req.Password) < 6 {
		fail(w, http.StatusBadRequest, "密码至少 6 位")
		return
	}
	if tooLong(req.Password, maxPassword) {
		fail(w, http.StatusBadRequest, "密码最长 64 位")
		return
	}
	acc, err := a.store.CreateStaffAccount(username, req.Password)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	// 支持分配角色（默认为员工；仅超级管理员本人在此上下文）
	role := strings.TrimSpace(req.Role)
	if role != "" && role != "staff" {
		if err := a.store.UpdateAccountRole(acc.ID, role); err == nil {
			acc.Role = role
		}
	}
	ok(w, acc)
}

// DELETE /api/v1/admin/accounts/{id} 销毁账号
func (a *API) adminDeleteAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cur := accountFrom(r)
	if cur != nil && cur.ID == id {
		fail(w, http.StatusBadRequest, "不能删除当前登录账号")
		return
	}
	target, err := a.store.GetAdminAccountByID(id)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if target == nil {
		fail(w, http.StatusNotFound, "账号不存在")
		return
	}
	if target.Role == "admin" {
		fail(w, http.StatusBadRequest, "不能销毁超级管理员账号")
		return
	}
	if err := a.store.DeleteAdminAccount(id); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, map[string]string{"id": id})
}

type accountRoleReq struct {
	Role string `json:"role"`
}

// PUT /api/v1/admin/accounts/{id}/role 修改账号权限（admin 分配；员工无此权限）
func (a *API) adminUpdateAccountRole(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cur := accountFrom(r)
	if cur != nil && cur.ID == id {
		fail(w, http.StatusBadRequest, "不能修改当前登录账号的权限")
		return
	}
	var req accountRoleReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if req.Role != "admin" && req.Role != "staff" {
		fail(w, http.StatusBadRequest, "角色无效（admin / staff）")
		return
	}
	target, err := a.store.GetAdminAccountByID(id)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if target == nil {
		fail(w, http.StatusNotFound, "账号不存在")
		return
	}
	if err := a.store.UpdateAccountRole(id, req.Role); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, map[string]string{"id": id, "role": req.Role})
}

type accountStatusReq struct {
	Status int `json:"status"`
}

// PUT /api/v1/admin/accounts/{id}/status 启用/停用账号
func (a *API) adminUpdateAccountStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cur := accountFrom(r)
	if cur != nil && cur.ID == id {
		fail(w, http.StatusBadRequest, "不能停用当前登录账号")
		return
	}
	var req accountStatusReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if req.Status != 0 && req.Status != 1 {
		fail(w, http.StatusBadRequest, "状态无效（0 停用 / 1 启用）")
		return
	}
	target, err := a.store.GetAdminAccountByID(id)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if target == nil {
		fail(w, http.StatusNotFound, "账号不存在")
		return
	}
	if err := a.store.UpdateAccountStatus(id, req.Status); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, map[string]string{"id": id, "status": fmt.Sprintf("%d", req.Status)})
}

// POST /api/v1/admin/devices/{id}/alarm 触发设备报警
func (a *API) adminDeviceAlarm(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	d := a.store.GetDevice(id)
	if d == nil {
		fail(w, http.StatusNotFound, "设备不存在")
		return
	}
	a.store.UpdateDeviceStatus(id, "alarm")
	am, exists := alarmMap[d.Type]
	if !exists {
		am = [2]string{"设备报警", "设备检测到异常情况，请及时处理"}
	}
	msg := a.store.AddMessage(id, "alarm", am[0], am[1])
	// 微信订阅消息推送（与设备报警上报一致）
	push := a.pushDeviceEvent(a.store.UserByID(d.UserID), am[0], am[1])
	ok(w, map[string]interface{}{"device": d, "message": msg, "push": push})
}

// DELETE /api/v1/admin/devices/{id} 移除设备
func (a *API) adminRemoveDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if a.store.GetDevice(id) == nil {
		fail(w, http.StatusNotFound, "设备不存在")
		return
	}
	// 管理端移除不校验 userId
	a.store.RemoveDevice("", id)
	ok(w, map[string]string{"id": id})
}

// DELETE /api/v1/admin/messages/{id} 删除消息
func (a *API) adminRemoveMessage(w http.ResponseWriter, r *http.Request) {
	// 简单实现：标记已读并返回（内存存储不提供物理删除，提供接口占位）
	id := r.PathValue("id")
	if !a.store.MarkRead(id) {
		fail(w, http.StatusNotFound, "消息不存在")
		return
	}
	ok(w, map[string]string{"id": id, "hint": "内存存储演示：已标记；正式版接数据库后物理删除"})
}

// GET /api/v1/admin/teams 团队列表（含成员与经营金额）
func (a *API) adminTeams(w http.ResponseWriter, r *http.Request) {
	teams, err := a.store.ListTeams()
	if err != nil {
		fail(w, http.StatusInternalServerError, "团队数据读取失败: "+err.Error())
		return
	}
	ok(w, teams)
}

// DELETE /api/v1/admin/teams/{id} 移除团队
func (a *API) adminRemoveTeam(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := a.store.DeleteTeam(id); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, map[string]string{"id": id})
}
