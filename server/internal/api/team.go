// team.go
// 团队接口（需登录）
// 建团资格：邀请人数 > 2 或所在团队经营金额 > 1w
// 团队可发布服务类商品（服务大类下），服务来源 = 团队名
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"socialserver/internal/store"
)

// 建团资格阈值
const (
	teamCreateMinInvites  = 3   // 邀请人数 > 2（即 ≥3）
	teamCreateMinBusiness = 1e4 // 所在团队经营金额 > 1w
)

// GET /api/v1/team/my 我的团队（队长或成员视角）
func (a *API) myTeam(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u == nil {
		fail(w, http.StatusUnauthorized, "未登录")
		return
	}
	team, err := a.store.GetTeamByPhone(u.Phone)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if team == nil {
		ok(w, nil)
		return
	}
	ok(w, team)
}

// ==================== 团队金库（服务分成 90% 入金库，仅团长可支配） ====================

type treasuryReq struct {
	Amount float64 `json:"amount"`
	Phone  string  `json:"phone"` // 转账目标成员手机号
}

// POST /api/v1/team/treasury/withdraw 团长提取金库到我的余额
func (a *API) treasuryWithdraw(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	var req treasuryReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if req.Amount <= 0 || req.Amount > 100000 {
		fail(w, http.StatusBadRequest, "提取金额无效（0 ~ 10 万元）")
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
	if u.Phone != team.OwnerPhone {
		fail(w, http.StatusForbidden, "仅有团长可以提取团队金库")
		return
	}
	if req.Amount > team.Treasury {
		fail(w, http.StatusBadRequest, "金库余额不足")
		return
	}
	updated, err := a.store.WithdrawTreasury(team, u, req.Amount)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, map[string]interface{}{"user": updated, "balance": updated.Balance})
}

// POST /api/v1/team/treasury/transfer 团长从金库向团队成员余额转账
func (a *API) treasuryTransfer(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	var req treasuryReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if req.Amount <= 0 || req.Amount > 100000 {
		fail(w, http.StatusBadRequest, "转账金额无效（0 ~ 10 万元）")
		return
	}
	req.Phone = strings.TrimSpace(req.Phone)
	if !phoneRe.MatchString(req.Phone) {
		fail(w, http.StatusBadRequest, "请填写正确的成员手机号")
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
	if u.Phone != team.OwnerPhone {
		fail(w, http.StatusForbidden, "仅有团长可以操作团队金库")
		return
	}
	if req.Amount > team.Treasury {
		fail(w, http.StatusBadRequest, "金库余额不足")
		return
	}
	target, err := a.store.TransferTreasury(team, u, req.Phone, req.Amount)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, map[string]interface{}{"user": target, "balance": target.Balance})
}

// GET /api/v1/team/treasury/logs 我的团队金库流水
func (a *API) treasuryLogs(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	team, err := a.store.GetTeamByPhone(u.Phone)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if team == nil {
		fail(w, http.StatusForbidden, "您不在任何团队中")
		return
	}
	logs, err := a.store.ListTreasuryLogs(team.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, logs)
}

// POST /api/v1/team/join 加入团队成为成员（需不在任何团队中）
func (a *API) joinTeam(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u == nil {
		fail(w, http.StatusUnauthorized, "未登录")
		return
	}
	var req struct {
		TeamID string `json:"teamId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if strings.TrimSpace(req.TeamID) == "" {
		fail(w, http.StatusBadRequest, "请输入团队 ID")
		return
	}
	if tooLong(req.TeamID, maxTeamID) {
		fail(w, http.StatusBadRequest, "团队 ID 不正确")
		return
	}
	team, err := a.store.JoinTeam(req.TeamID, u)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, team)
}

type createTeamReq struct {
	Name        string  `json:"name"`
	BusinessAmt float64 `json:"businessAmount"` // 所在团队经营金额（若在团队中）
}

// POST /api/v1/team/create 创建团队
// 资格：邀请人数 > 2，或所在团队经营金额 > 1w（邀请人数由服务器依据真实邀请关系统计，不信任客户端上报）
func (a *API) createTeam(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u == nil {
		fail(w, http.StatusUnauthorized, "未登录")
		return
	}
	var req createTeamReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		fail(w, http.StatusBadRequest, "团队名称不能为空")
		return
	}
	if tooLong(req.Name, maxTeamName) {
		fail(w, http.StatusBadRequest, "团队名称最长 20 个字符")
		return
	}
	if req.BusinessAmt < 0 || req.BusinessAmt > 10000000 {
		fail(w, http.StatusBadRequest, "经营金额异常")
		return
	}
	invitees, err := a.store.ListInvitees(u.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(invitees) < teamCreateMinInvites && req.BusinessAmt <= teamCreateMinBusiness {
		fail(w, http.StatusForbidden,
			fmt.Sprintf("暂不具备建团资格：邀请好友需超过 %d 人，或所在团队经营金额超过 %d 元", teamCreateMinInvites-1, int(teamCreateMinBusiness)))
		return
	}
	// 已在团队中不可再直接建团（团员建新团需走申请-团长审核流程）
	existing, err := a.store.GetTeamByPhone(u.Phone)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing != nil {
		fail(w, http.StatusBadRequest, "您已在团队中，创建新团请提交申请，由团长审核通过后生效")
		return
	}
	team, err := a.store.CreateTeam(req.Name, u.Phone, u.NickName)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, team)
}

// POST /api/v1/team/apply-create 团员申请创建新团（需现任团长审核）
func (a *API) applyCreateTeam(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u == nil {
		fail(w, http.StatusUnauthorized, "未登录")
		return
	}
	var req createTeamReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		fail(w, http.StatusBadRequest, "团队名称不能为空")
		return
	}
	if tooLong(req.Name, maxTeamName) {
		fail(w, http.StatusBadRequest, "团队名称最长 20 个字符")
		return
	}
	if req.BusinessAmt < 0 || req.BusinessAmt > 10000000 {
		fail(w, http.StatusBadRequest, "经营金额异常")
		return
	}
	invitees, err := a.store.ListInvitees(u.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(invitees) < teamCreateMinInvites && req.BusinessAmt <= teamCreateMinBusiness {
		fail(w, http.StatusForbidden,
			fmt.Sprintf("暂不具备建团资格：邀请好友需超过 %d 人，或所在团队经营金额超过 %d 元", teamCreateMinInvites-1, int(teamCreateMinBusiness)))
		return
	}
	current, err := a.store.GetTeamByPhone(u.Phone)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if current == nil {
		fail(w, http.StatusBadRequest, "您不在团队中，可直接创建团队，无需申请")
		return
	}
	if current.OwnerPhone == u.Phone {
		fail(w, http.StatusBadRequest, "您是团长，无需申请")
		return
	}
	reqObj, err := a.store.CreateTeamRequest(u, req.Name, current)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, reqObj)
}

// GET /api/v1/team/requests/my 我提交的建团申请
func (a *API) myTeamRequests(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u == nil {
		fail(w, http.StatusUnauthorized, "未登录")
		return
	}
	reqs, err := a.store.ListTeamRequestsByRequester(u.Phone)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, reqs)
}

// GET /api/v1/team/requests/inbox 待我审核的建团申请（团长收件箱）
func (a *API) teamRequestInbox(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u == nil {
		fail(w, http.StatusUnauthorized, "未登录")
		return
	}
	team, err := a.store.GetTeamByPhone(u.Phone)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if team == nil {
		ok(w, []store.TeamCreateRequest{})
		return
	}
	if team.OwnerPhone != u.Phone {
		ok(w, []store.TeamCreateRequest{}) // 非团长无收件箱
		return
	}
	reqs, err := a.store.ListPendingTeamRequestsForTeam(team.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, reqs)
}

// POST /api/v1/team/requests/{id}/approve 团长审核通过
func (a *API) approveTeamRequest(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u == nil {
		fail(w, http.StatusUnauthorized, "未登录")
		return
	}
	id := r.PathValue("id")
	reqObj, err := a.store.GetTeamRequest(id)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if reqObj == nil {
		fail(w, http.StatusNotFound, "申请不存在")
		return
	}
	// 仅申请人所在团队团长可审核
	team, err := a.store.GetTeam(reqObj.CurrentTeamID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if team == nil || team.OwnerPhone != u.Phone {
		fail(w, http.StatusForbidden, "仅团长可审核该申请")
		return
	}
	newTeam, updated, err := a.store.ApproveTeamRequest(id, u.Phone)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, map[string]interface{}{"team": newTeam, "request": updated})
}

// POST /api/v1/team/requests/{id}/reject 团长审核驳回
func (a *API) rejectTeamRequest(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u == nil {
		fail(w, http.StatusUnauthorized, "未登录")
		return
	}
	id := r.PathValue("id")
	reqObj, err := a.store.GetTeamRequest(id)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if reqObj == nil {
		fail(w, http.StatusNotFound, "申请不存在")
		return
	}
	team, err := a.store.GetTeam(reqObj.CurrentTeamID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if team == nil || team.OwnerPhone != u.Phone {
		fail(w, http.StatusForbidden, "仅团长可审核该申请")
		return
	}
	updated, err := a.store.RejectTeamRequest(id, u.Phone)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, updated)
}

// POST /api/v1/team/business 累计团队经营金额（团队服务商品订单完成后上报）
// 支持按来源团队名（sourceTeam）累计；未提供时按当前用户所在团队累计
func (a *API) addTeamBusiness(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u == nil {
		fail(w, http.StatusUnauthorized, "未登录")
		return
	}
	var req struct {
		Amount   float64 `json:"amount"`
		TeamName string  `json:"teamName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if req.Amount <= 0 || req.Amount > 1000000 {
		fail(w, http.StatusBadRequest, "金额无效（单笔上限 100 万元）")
		return
	}
	if strings.TrimSpace(req.TeamName) != "" {
		if err := a.store.AddTeamBusinessByName(req.TeamName, req.Amount); err != nil {
			fail(w, http.StatusBadRequest, err.Error())
			return
		}
	} else if err := a.store.AddTeamBusiness(u.Phone, req.Amount); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	team, err := a.store.GetTeamByPhone(u.Phone)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, team)
}

type publishServiceReq struct {
	Name       string                     `json:"name"`
	Price      float64                    `json:"price"`
	OriginalPrice float64                 `json:"originalPrice"`
	Desc       string                     `json:"desc"`
	Emoji      string                     `json:"emoji"`
	Category   string                     `json:"category"` // 服务大类下的二级类目 ID
	Tags       []string                   `json:"tags"`
	Detail     []string                   `json:"detail"`
	Attributes []store.ProductAttribute   `json:"attributes"`
}

// POST /api/v1/team/products 团队发布服务类商品
// 校验：团队成员身份 + 类目必须是服务大类
func (a *API) teamPublishService(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u == nil {
		fail(w, http.StatusUnauthorized, "未登录")
		return
	}
	team, err := a.store.GetTeamByPhone(u.Phone)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if team == nil {
		fail(w, http.StatusForbidden, "仅团队可发布服务商品，请先创建或加入团队")
		return
	}
	var req publishServiceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || req.Price <= 0 {
		fail(w, http.StatusBadRequest, "服务名称与价格必填")
		return
	}
	if tooLong(req.Name, maxProductName) {
		fail(w, http.StatusBadRequest, "服务名称最长 60 个字符")
		return
	}
	if req.Price > 999999 || req.Price <= 0 {
		fail(w, http.StatusBadRequest, "服务价格须在 0 ~ 999999 元之间")
		return
	}
	if req.OriginalPrice < 0 || req.OriginalPrice > 999999 {
		fail(w, http.StatusBadRequest, "服务原价超出合理范围（0 ~ 999999）")
		return
	}
	if tooLong(req.Desc, maxDesc) {
		fail(w, http.StatusBadRequest, "服务描述最长 200 个字符")
		return
	}
	if tooLong(req.Emoji, 8) {
		fail(w, http.StatusBadRequest, "服务图标过长")
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
	// 类目必须是服务大类（含二级）
	cat, err := a.store.GetCategory(req.Category)
	if err != nil || cat == nil {
		fail(w, http.StatusBadRequest, "服务类目不存在")
		return
	}
	if !store.IsServiceCategory(cat) {
		fail(w, http.StatusForbidden, "非服务类目下不能发布服务商品")
		return
	}
	if req.OriginalPrice < req.Price {
		req.OriginalPrice = req.Price
	}
	p := store.Product{
		ID:            "sp" + fmt.Sprintf("%03d", time.Now().UnixNano()%1000000),
		Name:          req.Name,
		Desc:          req.Desc,
		Price:         req.Price,
		OriginalPrice: req.OriginalPrice,
		Emoji:         req.Emoji,
		Colors:        []string{"#0EA5E9", "#075985"},
		Sales:         0,
		Category:      req.Category,
		Tags:          req.Tags,
		Detail:        req.Detail,
		Attributes:    req.Attributes,
		Service:       true,
		SourceTeam:    team.Name,
	}
	if err := a.store.UpsertProduct(p); err != nil {
		fail(w, http.StatusInternalServerError, "服务发布失败: "+err.Error())
		return
	}
	ok(w, p)
}

// ==================== 团队入团邀请（团长邀请 → 对方同意后入团） ====================

// GET /api/v1/team/invites/candidates 邀请候选：我邀请的好友（附团队状态，供优先选取）
func (a *API) teamInviteCandidates(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u == nil {
		fail(w, http.StatusUnauthorized, "未登录")
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
		fail(w, http.StatusForbidden, "仅团长可以邀请成员")
		return
	}
	invitees, err := a.store.ListInvitees(u.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := []map[string]interface{}{}
	for _, inv := range invitees {
		item := map[string]interface{}{
			"userId":    inv.UserID,
			"phone":     inv.Phone,
			"nickName":  inv.NickName,
			"joinTime":  inv.JoinTime,
			"inTeam":    false,
			"inMyTeam":  false,
			"teamName":  "",
			"pending":   false,
		}
		t, err := a.store.GetTeamByPhone(inv.Phone)
		if err == nil && t != nil {
			item["inTeam"] = true
			item["teamName"] = t.Name
			if team != nil && t.ID == team.ID {
				item["inMyTeam"] = true
			}
		}
		// 是否已有本团队待处理邀请
		pen, _ := a.store.HasPendingTeamInvite(team.ID, inv.Phone)
		item["pending"] = pen
		out = append(out, item)
	}
	ok(w, out)
}

// POST /api/v1/team/invites 邀请成员入团（仅团长；被邀请人同意后入团）
func (a *API) createTeamInvite(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u == nil {
		fail(w, http.StatusUnauthorized, "未登录")
		return
	}
	var req struct {
		Phone string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	req.Phone = strings.TrimSpace(req.Phone)
	if !validPhone(req.Phone) {
		fail(w, http.StatusBadRequest, "请填写正确的手机号")
		return
	}
	if req.Phone == u.Phone {
		fail(w, http.StatusBadRequest, "不能邀请自己入团")
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
		fail(w, http.StatusForbidden, "仅团长可以邀请成员")
		return
	}
	invitee := a.store.UserByPhone(req.Phone)
	if invitee == nil {
		fail(w, http.StatusBadRequest, "该手机号未注册，无法邀请")
		return
	}
	inv, err := a.store.CreateTeamInvite(team, u, invitee)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, inv)
}

// GET /api/v1/team/invites/outbox 我发出的全部邀请（团长视角，含历史状态）
func (a *API) teamInviteOutbox(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u == nil {
		fail(w, http.StatusUnauthorized, "未登录")
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
	list, err := a.store.ListTeamInvites(team.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, list)
}

// GET /api/v1/team/invites/inbox 我收到的待处理邀请（同意后入团）
func (a *API) teamInviteInbox(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u == nil {
		fail(w, http.StatusUnauthorized, "未登录")
		return
	}
	list, err := a.store.ListTeamInviteInbox(u.Phone)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, list)
}

// POST /api/v1/team/invites/{id}/accept 接受邀请并入团
func (a *API) acceptTeamInvite(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u == nil {
		fail(w, http.StatusUnauthorized, "未登录")
		return
	}
	id := r.PathValue("id")
	inv, team, err := a.store.AcceptTeamInvite(id, u)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, map[string]interface{}{"invite": inv, "team": team})
}

// POST /api/v1/team/invites/{id}/reject 拒绝邀请
func (a *API) rejectTeamInvite(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u == nil {
		fail(w, http.StatusUnauthorized, "未登录")
		return
	}
	id := r.PathValue("id")
	inv, err := a.store.RejectTeamInvite(id, u.Phone)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, inv)
}

// POST /api/v1/team/invites/{id}/cancel 团长取消待处理邀请
func (a *API) cancelTeamInvite(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u == nil {
		fail(w, http.StatusUnauthorized, "未登录")
		return
	}
	id := r.PathValue("id")
	inv, err := a.store.CancelTeamInvite(id, u.Phone)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, inv)
}

// GET /api/v1/users/search 手机号查询用户（邀请入团用；返回是否已注册/是否已在团队）
func (a *API) searchUser(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u == nil {
		fail(w, http.StatusUnauthorized, "未登录")
		return
	}
	phone := strings.TrimSpace(r.URL.Query().Get("phone"))
	if !validPhone(phone) {
		fail(w, http.StatusBadRequest, "请填写正确的手机号")
		return
	}
	target := a.store.UserByPhone(phone)
	if target == nil {
		fail(w, http.StatusNotFound, "该手机号未注册，无法邀请")
		return
	}
	myTeam, _ := a.store.GetTeamByPhone(u.Phone)
	targetTeam, _ := a.store.GetTeamByPhone(phone)
	inMyTeam := false
	teamName := ""
	if targetTeam != nil {
		teamName = targetTeam.Name
		if myTeam != nil && targetTeam.ID == myTeam.ID {
			inMyTeam = true
		}
	}
	pending := false
	if myTeam != nil {
		pen, _ := a.store.HasPendingTeamInvite(myTeam.ID, phone)
		pending = pen
	}
	ok(w, map[string]interface{}{
		"phone":    target.Phone,
		"nickName": target.NickName,
		"inTeam":   targetTeam != nil,
		"inMyTeam": inMyTeam,
		"teamName": teamName,
		"pending":  pending,
	})
}

