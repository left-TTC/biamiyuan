// support.go
// 客服接口
// 分配规则：服务商品（service + sourceTeam）→ 团队指定客服成员（teams.support_member_phone）；
//           普通商品 / 官方服务 → 后台客服（管理工作台回复）
package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

// ==================== 用户端 ====================

// GET /api/v1/support/tickets 我的客服会话
func (a *API) listSupportTickets(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	list, err := a.store.ListSupportTicketsByUser(u.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, list)
}

type createSupportReq struct {
	Subject     string `json:"subject"` // 问题标题（必填）
	ProductID   string `json:"productId"`
	ProductName string `json:"productName"`
	Message     string `json:"message"` // 首条消息
}

// POST /api/v1/support/tickets 创建客服会话（自动分配客服）
func (a *API) createSupportTicket(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	var req createSupportReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	req.Subject = strings.TrimSpace(req.Subject)
	req.Message = strings.TrimSpace(req.Message)
	if req.Subject == "" {
		fail(w, http.StatusBadRequest, "请填写问题标题")
		return
	}
	if tooLong(req.Subject, maxSubject) {
		fail(w, http.StatusBadRequest, "问题标题最长 50 个字符")
		return
	}
	if req.Message == "" {
		fail(w, http.StatusBadRequest, "请填写问题描述")
		return
	}
	if tooLong(req.Message, maxMsgContent) {
		fail(w, http.StatusBadRequest, "问题描述最长 1000 个字符")
		return
	}
	if tooLong(req.ProductID, maxTeamID) {
		fail(w, http.StatusBadRequest, "关联商品参数错误")
		return
	}
	if tooLong(req.ProductName, maxProductName) {
		fail(w, http.StatusBadRequest, "关联商品名称过长")
		return
	}
	// 关联商品：有商品 ID 时以服务器商品信息为准
	productName := req.ProductName
	service := false
	sourceTeam := ""
	if strings.TrimSpace(req.ProductID) != "" {
		if p, err := a.store.GetProduct(req.ProductID); err == nil && p != nil {
			productName = p.Name
			service = p.Service
			sourceTeam = p.SourceTeam
		}
	}
	ticket, err := a.store.CreateSupportTicket(u, req.Subject, req.ProductID, productName,
		service, sourceTeam, req.Message)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, ticket)
}

// GET /api/v1/support/tickets/{id} 会话详情（含消息）
// 权限：会话归属用户本人，或该会话被分配的团队客服成员
func (a *API) getSupportTicket(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	id := r.PathValue("id")
	t, err := a.store.GetSupportTicket(id)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if t == nil {
		fail(w, http.StatusNotFound, "会话不存在")
		return
	}
	isOwner := t.UserID == u.ID
	isAssignee := t.AssigneeType == "team" && t.AssigneePhone == u.Phone
	if !isOwner && !isAssignee {
		fail(w, http.StatusForbidden, "无权查看该会话")
		return
	}
	msgs, err := a.store.ListSupportMessages(id)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	// 用户查看 → 客服回复标记已读；客服查看 → 用户消息标记已读
	if isOwner {
		_ = a.store.MarkSupportTicketRead(id, true)
	} else {
		_ = a.store.MarkSupportTicketRead(id, false)
	}
	ok(w, map[string]interface{}{"ticket": t, "messages": msgs})
}

type sendSupportMsgReq struct {
	Content string `json:"content"`
}

// POST /api/v1/support/tickets/{id}/messages 用户发送消息
func (a *API) sendSupportMessage(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	var req sendSupportMsgReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	content, msgErr := validSupportContent(req.Content)
	if msgErr != "" {
		fail(w, http.StatusBadRequest, msgErr)
		return
	}
	msg, err := a.store.SendSupportMessage(u.ID, r.PathValue("id"), content)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, msg)
}

// ==================== 团队客服成员 ====================

// GET /api/v1/support/inbox 我的客服收件箱（我是某团队指定客服）
func (a *API) mySupportInbox(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	list, err := a.store.ListSupportTicketsByAssignee(u.Phone)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, list)
}

// POST /api/v1/support/tickets/{id}/reply 团队客服回复（须是该会话被分配的客服成员）
func (a *API) replySupportTicket(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	id := r.PathValue("id")
	t, err := a.store.GetSupportTicket(id)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if t == nil {
		fail(w, http.StatusNotFound, "会话不存在")
		return
	}
	if t.AssigneeType != "team" || t.AssigneePhone != u.Phone {
		fail(w, http.StatusForbidden, "该会话未分配给您")
		return
	}
	var req sendSupportMsgReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	content, msgErr := validSupportContent(req.Content)
	if msgErr != "" {
		fail(w, http.StatusBadRequest, msgErr)
		return
	}
	msg, err := a.store.SendSupportReply(id, "team", u.NickName, content)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, msg)
}

// POST /api/v1/support/tickets/{id}/close 团队客服关闭会话（须为该会话被分配客服）
func (a *API) closeSupportTicket(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	id := r.PathValue("id")
	t, err := a.store.GetSupportTicket(id)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if t == nil {
		fail(w, http.StatusNotFound, "会话不存在")
		return
	}
	if t.AssigneeType != "team" || t.AssigneePhone != u.Phone {
		fail(w, http.StatusForbidden, "该会话未分配给您")
		return
	}
	closed, err := a.store.CloseSupportTicket(id)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, closed)
}

// POST /api/v1/team/support-member 团长为团队指定客服成员（接收/回复团队服务会话）
func (a *API) setTeamSupportMember(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	var req struct {
		MemberPhone string `json:"memberPhone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if !validPhone(req.MemberPhone) {
		fail(w, http.StatusBadRequest, "请填写正确的客服成员手机号")
		return
	}
	team, err := a.store.GetTeamByPhone(u.Phone)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if team == nil {
		fail(w, http.StatusForbidden, "您不在任何团队中")
		return
	}
	if team.OwnerPhone != u.Phone {
		fail(w, http.StatusForbidden, "仅团队团长可指定客服成员")
		return
	}
	updated, err := a.store.SetTeamSupportMember(team.ID, req.MemberPhone)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, updated)
}


// ==================== 管理端（后台客服工作台） ====================

// GET /api/v1/admin/support/tickets?status= 全部客服会话
func (a *API) adminListSupportTickets(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	list, err := a.store.ListAllSupportTickets(status)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, list)
}

// GET /api/v1/admin/support/tickets/{id} 会话详情（含消息）
func (a *API) adminGetSupportTicket(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := a.store.GetSupportTicket(id)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if t == nil {
		fail(w, http.StatusNotFound, "会话不存在")
		return
	}
	msgs, err := a.store.ListSupportMessages(id)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	// 管理端查看 → 用户消息标记已读
	_ = a.store.MarkSupportTicketRead(id, false)
	ok(w, map[string]interface{}{"ticket": t, "messages": msgs})
}

// POST /api/v1/admin/support/tickets/{id}/reply 后台客服回复
func (a *API) adminReplySupport(w http.ResponseWriter, r *http.Request) {
	acc := accountFrom(r)
	id := r.PathValue("id")
	t, err := a.store.GetSupportTicket(id)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if t == nil {
		fail(w, http.StatusNotFound, "会话不存在")
		return
	}
	var req sendSupportMsgReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	content, msgErr := validSupportContent(req.Content)
	if msgErr != "" {
		fail(w, http.StatusBadRequest, msgErr)
		return
	}
	senderName := "客服"
	if acc != nil {
		senderName = acc.Username
	}
	msg, err := a.store.SendSupportReply(id, "admin", senderName, content)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, msg)
}

// POST /api/v1/admin/support/tickets/{id}/close 后台关闭会话
func (a *API) adminCloseSupport(w http.ResponseWriter, r *http.Request) {
	t, err := a.store.CloseSupportTicket(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, t)
}

