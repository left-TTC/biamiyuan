// cart.go
// 用户购物车接口（需登录）与管理端购物车查看
package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"socialserver/internal/store"
)

// GET /api/v1/cart 获取当前用户购物车（[{productId, quantity}]）
func (a *API) getCart(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.ListCartItems(userFrom(r).ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, "购物车读取失败: "+err.Error())
		return
	}
	ok(w, items)
}

type syncCartReq struct {
	Items []store.CartItem `json:"items"`
}

// POST /api/v1/cart/sync 全量同步当前用户购物车（以小程序本地购物车为准）
func (a *API) syncCart(w http.ResponseWriter, r *http.Request) {
	var req syncCartReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if req.Items == nil {
		req.Items = []store.CartItem{}
	}
	if len(req.Items) > 100 {
		fail(w, http.StatusBadRequest, "购物车商品数过多（最多 100 件）")
		return
	}
	for _, it := range req.Items {
		if strings.TrimSpace(it.ProductID) == "" {
			fail(w, http.StatusBadRequest, "购物车商品数据无效")
			return
		}
		if it.Quantity <= 0 || it.Quantity > 999 {
			fail(w, http.StatusBadRequest, "商品数量须在 1 ~ 999 之间")
			return
		}
	}
	if err := a.store.SyncCart(userFrom(r).ID, req.Items); err != nil {
		fail(w, http.StatusInternalServerError, "购物车同步失败: "+err.Error())
		return
	}
	ok(w, req.Items)
}

type updateCartItemReq struct {
	Quantity int `json:"quantity"`
}

// PUT /api/v1/cart/items/{productId} 更新购物车中商品数量
func (a *API) updateCartItem(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("productId")
	var req updateCartItemReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if req.Quantity <= 0 || req.Quantity > 999 {
		fail(w, http.StatusBadRequest, "数量必须大于 0 且不超过 999")
		return
	}
	if err := a.store.UpdateCartItemQuantity(userFrom(r).ID, productID, req.Quantity); err != nil {
		fail(w, http.StatusInternalServerError, "购物车更新失败: "+err.Error())
		return
	}
	ok(w, map[string]interface{}{"productId": productID, "quantity": req.Quantity})
}

// DELETE /api/v1/cart/items/{productId} 从购物车移除商品
func (a *API) deleteCartItem(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("productId")
	if err := a.store.DeleteCartItem(userFrom(r).ID, productID); err != nil {
		fail(w, http.StatusInternalServerError, "购物车移除失败: "+err.Error())
		return
	}
	ok(w, map[string]string{"productId": productID})
}

// GET /api/v1/admin/carts 所有用户购物车（管理端查看）
func (a *API) adminCarts(w http.ResponseWriter, r *http.Request) {
	carts, err := a.store.ListAllCarts()
	if err != nil {
		fail(w, http.StatusInternalServerError, "购物车数据读取失败: "+err.Error())
		return
	}
	if carts == nil {
		carts = []store.UserCart{}
	}
	ok(w, carts)
}
