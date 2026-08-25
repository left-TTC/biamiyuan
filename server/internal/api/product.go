// product.go
// 商品与分类接口（公开接口，无需登录）
package api

import "net/http"

// GET /api/v1/categories 商品分类列表
func (a *API) listCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := a.store.ListCategories()
	if err != nil {
		fail(w, http.StatusInternalServerError, "分类数据读取失败: "+err.Error())
		return
	}
	ok(w, cats)
}

// GET /api/v1/products 商品列表
//
//	可选参数：category（分类ID）、keyword（搜索关键词）
func (a *API) listProducts(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	keyword := r.URL.Query().Get("keyword")
	products, err := a.store.ListProducts(category, keyword)
	if err != nil {
		fail(w, http.StatusInternalServerError, "商品数据读取失败: "+err.Error())
		return
	}
	ok(w, products)
}

// GET /api/v1/products/{id} 商品详情
func (a *API) getProduct(w http.ResponseWriter, r *http.Request) {
	p, err := a.store.GetProduct(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusInternalServerError, "商品数据读取失败: "+err.Error())
		return
	}
	if p == nil {
		fail(w, http.StatusNotFound, "商品不存在")
		return
	}
	ok(w, p)
}
