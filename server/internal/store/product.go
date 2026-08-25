// product.go
// 商品与分类的数据结构定义
// 数据存储与查询见 catalog.go（SQLite 持久化，可动态管理）
package store

import "time"

// Category 商品分类（两级类目）
// 一级类目：ParentID 为空；二级类目：ParentID 指向所属一级类目
// IsService=true 表示服务大类（固定的"服务"一级类目：仅后台与团队可发布服务类商品）
type Category struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ParentID  string `json:"parentId"` // 空表示一级类目
	Sort      int    `json:"sort"`
	IsService bool   `json:"isService"`
}

// ProductAttribute 商品内置属性（由创建者定义）
// 例如衣服：{ Name: "尺码", Values: ["S", "M", "L", "XL"] }
// 购买时需为每个属性选择一个取值；普通商品由后台定义，服务商品由团队定义
type ProductAttribute struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

// Product 商品
type Product struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Desc          string             `json:"desc"`
	Price         float64            `json:"price"`
	OriginalPrice float64            `json:"originalPrice"`
	Emoji         string             `json:"emoji"`
	Colors        []string           `json:"colors"`
	Images        []string           `json:"images"` // 商品图片 URL 数组（第一张为列表头像）
	Sales         int                `json:"sales"`
	Category      string             `json:"category"`
	Tags          []string           `json:"tags"`
	Detail        []string           `json:"detail"`
	Attributes    []ProductAttribute `json:"attributes,omitempty"` // 内置属性（下单时需选择）
	Service       bool               `json:"service,omitempty"`    // 服务类商品（服务大类下）
	SourceTeam    string             `json:"sourceTeam,omitempty"` // 服务来源（团队名称 / 官方）
	UpdatedAt     int64              `json:"updatedAt,omitempty"`
}

// timeNow 当前毫秒时间戳
func timeNow() int64 {
	return time.Now().UnixMilli()
}
