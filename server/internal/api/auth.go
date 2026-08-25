// auth.go
// 认证：手机号 + 验证码登录、Bearer Token 鉴权
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"socialserver/internal/store"
)

// demoCode 演示验证码
// 真实环境：接入短信服务商后改为服务端随机生成并通过短信下发
const demoCode = "12345"

type ctxKey int

const userCtxKey ctxKey = 0

// userFrom 从请求上下文获取当前登录用户
func userFrom(r *http.Request) *store.User {
	u, _ := r.Context().Value(userCtxKey).(*store.User)
	return u
}

// auth 鉴权中间件：校验请求头 Authorization: Bearer <token>
func (a *API) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			fail(w, http.StatusUnauthorized, "未登录")
			return
		}
		u := a.store.UserByToken(token)
		if u == nil {
			fail(w, http.StatusUnauthorized, "登录已失效")
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey, u)
		next(w, r.WithContext(ctx))
	}
}

type codeReq struct {
	Phone string `json:"phone"`
}

// POST /api/v1/auth/code 发送验证码（演示：固定 12345）
func (a *API) sendCode(w http.ResponseWriter, r *http.Request) {
	var req codeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if !validPhone(req.Phone) {
		fail(w, http.StatusBadRequest, "请填写正确的手机号")
		return
	}
	if tooLong(req.Phone, 11) {
		fail(w, http.StatusBadRequest, "手机号长度不正确")
		return
	}
	a.store.SaveCode(req.Phone, demoCode)
	ok(w, map[string]interface{}{
		"phone": req.Phone,
		"hint":  "演示环境验证码固定为 " + demoCode,
	})
}

type loginReq struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

// POST /api/v1/auth/register 手机号注册（演示：验证码怎么填都通过）
// 账号未注册时才能注册，注册后签发 token
func (a *API) register(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if !validPhone(req.Phone) {
		fail(w, http.StatusBadRequest, "请填写正确的手机号")
		return
	}
	if a.store.UserByPhone(req.Phone) != nil {
		fail(w, http.StatusConflict, "该手机号已注册，请直接登录")
		return
	}
	u := a.store.CreateUser(req.Phone)
	if u == nil {
		fail(w, http.StatusConflict, "该手机号已注册，请直接登录")
		return
	}
	token := a.store.CreateToken(u.ID)
	ok(w, map[string]interface{}{"token": token, "user": u})
}

// POST /api/v1/auth/login 手机号登录（演示：验证码怎么填都通过；账号须已注册）
func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if !validPhone(req.Phone) {
		fail(w, http.StatusBadRequest, "请填写正确的手机号")
		return
	}
	u := a.store.UserByPhone(req.Phone)
	if u == nil {
		fail(w, http.StatusNotFound, "该手机号未注册，请先注册")
		return
	}
	token := a.store.CreateToken(u.ID)
	ok(w, map[string]interface{}{"token": token, "user": u})
}

// GET /api/v1/user/me 当前用户信息
func (a *API) userMe(w http.ResponseWriter, r *http.Request) {
	ok(w, userFrom(r))
}

type wxLoginReq struct {
	Code string `json:"code"` // wx.login 返回的临时 code
}

// POST /api/v1/auth/wx-login 用 wx.login 的 code 换取 openid 并绑定当前用户
//
// 未配置微信凭据时：模拟 openid（sim_<userID>），保证接口链路可用
func (a *API) wxLogin(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r)
	var req wxLoginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if strings.TrimSpace(req.Code) == "" {
		fail(w, http.StatusBadRequest, "code 不能为空")
		return
	}
	if tooLong(req.Code, 256) {
		fail(w, http.StatusBadRequest, "code 长度不正确")
		return
	}

	if a.wechat == nil || !a.wechat.Configured() {
		openid := "sim_" + user.ID
		a.store.SetOpenID(user.ID, openid)
		ok(w, map[string]interface{}{
			"openid": openid,
			"mode":   "simulate",
			"hint":   "未配置 WECHAT_APPID/SECRET，使用模拟 openid；配置后自动走真实 code2Session",
		})
		return
	}

	openid, err := a.wechat.Code2Session(req.Code)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.store.SetOpenID(user.ID, openid)
	ok(w, map[string]interface{}{"openid": openid, "mode": "real"})
}
