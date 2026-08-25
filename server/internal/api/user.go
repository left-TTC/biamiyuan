// user.go
// 用户资料接口：更新昵称 / 头像
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"socialserver/internal/store"
)

type updateProfileReq struct {
	NickName  string `json:"nickName"`
	AvatarURL string `json:"avatarUrl"`
}

// PUT /api/v1/user/profile 更新当前用户资料（昵称 / 头像 URL）
// 昵称最长 20 字；昵称与头像 URL 至少提供一个
func (a *API) updateProfile(w http.ResponseWriter, r *http.Request) {
	var req updateProfileReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	req.NickName = strings.TrimSpace(req.NickName)
	req.AvatarURL = strings.TrimSpace(req.AvatarURL)
	if req.NickName == "" && req.AvatarURL == "" {
		fail(w, http.StatusBadRequest, "昵称和头像不能同时为空")
		return
	}
	// 昵称长度限制（按字符数，避免 emoji 被截断）
	if req.NickName != "" && tooLong(req.NickName, maxNickName) {
		fail(w, http.StatusBadRequest, "昵称最长 20 个字符")
		return
	}
	if tooLong(req.AvatarURL, 255) {
		fail(w, http.StatusBadRequest, "头像地址过长")
		return
	}
	user := a.store.UpdateUserProfile(userFrom(r).ID, req.NickName, req.AvatarURL)
	if user == nil {
		fail(w, http.StatusNotFound, "用户不存在")
		return
	}
	ok(w, user)
}

// POST /api/v1/user/bind-inviter 被邀请人绑定邀请人（输入邀请人的推广码）
// 绑定关系持久化到服务器 users.invited_by；幂等：仅首次绑定生效
func (a *API) bindInviter(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	req.Code = strings.TrimSpace(req.Code)
	if req.Code == "" {
		fail(w, http.StatusBadRequest, "请输入邀请码")
		return
	}
	if tooLong(req.Code, 20) {
		fail(w, http.StatusBadRequest, "邀请码不正确")
		return
	}
	a.store.SettleDueCommissions()
	user, err := a.store.BindInviter(u.ID, req.Code)
	if err != nil {
		log.Printf("[invite] 用户 %s(%s) 绑定邀请码 %s 失败: %v", u.NickName, u.Phone, req.Code, err)
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	log.Printf("[invite] 用户 %s(%s) 已绑定邀请人 %s（%s）", u.NickName, u.Phone, user.InvitedByName, user.InvitedBy)
	ok(w, user)
}

// GET /api/v1/user/invitees 我邀请的好友（服务端持久化的邀请关系 + 消费/佣金统计）
func (a *API) myInvitees(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	a.store.SettleDueCommissions()
	list, err := a.store.ListInvitees(u.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, list)
}

// GET /api/v1/user/commissions 我的佣金明细（pending 待结算 / settled 已到账 / cancelled 已取消）
func (a *API) myCommissions(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	a.store.SettleDueCommissions()
	list, err := a.store.ListCommissions(u.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, map[string]interface{}{
		"list":      list,
		"settleDays": store.CommissionSettleDays(),
	})
}

// POST /api/v1/user/commission/demo 单机演示：模拟好友下单产生佣金（无真实订单），
// 与真实佣金一致：延迟到无理由退货期满后才到账
func (a *API) demoCommission(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	var req struct {
		Amount float64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	c, err := a.store.CreateDemoCommission(u.ID, req.Amount)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, map[string]interface{}{
		"commission": c,
		"settleDays": store.CommissionSettleDays(),
	})
}

// POST /api/v1/user/avatar 上传用户头像（multipart，字段名 file）
//
//	保存到 uploads/ 目录，返回可访问的相对 URL，如 /uploads/avatar_xxx.jpg
func (a *API) uploadAvatar(w http.ResponseWriter, r *http.Request) {
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
	name := fmt.Sprintf("avatar_%d%s", time.Now().UnixNano(), ext)
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
