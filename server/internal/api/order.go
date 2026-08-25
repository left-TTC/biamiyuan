// order.go
// 收货地址 / 订单（支付+物流）/ 提现 接口
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"regexp"
	"strings"

	"socialserver/internal/store"
)

// phoneRe 中国大陆手机号
var phoneRe = regexp.MustCompile(`^1\d{10}$`)

// ==================== 收货地址 ====================

// GET /api/v1/addresses 我的收货地址列表
func (a *API) listAddresses(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	list, err := a.store.ListAddresses(u.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, list)
}

type addressReq struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	Region    string `json:"region"`
	Detail    string `json:"detail"`
	IsDefault bool   `json:"isDefault"`
}

func validAddressReq(req addressReq) string {
	if strings.TrimSpace(req.Name) == "" {
		return "请填写收货人"
	}
	if tooLong(req.Name, maxAddressName) {
		return "收货人最长 20 个字符"
	}
	if !phoneRe.MatchString(strings.TrimSpace(req.Phone)) {
		return "请填写正确的手机号"
	}
	if strings.TrimSpace(req.Region) == "" {
		return "请填写所在地区"
	}
	if tooLong(req.Region, maxRegion) {
		return "所在地区最长 60 个字符"
	}
	if strings.TrimSpace(req.Detail) == "" {
		return "请填写详细地址"
	}
	if tooLong(req.Detail, maxAddressDet) {
		return "详细地址最长 120 个字符"
	}
	return ""
}

// POST /api/v1/addresses 新增地址
func (a *API) saveAddress(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	var req addressReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if msg := validAddressReq(req); msg != "" {
		fail(w, http.StatusBadRequest, msg)
		return
	}
	addr, err := a.store.SaveAddress(u.ID, store.Address{
		Name:      strings.TrimSpace(req.Name),
		Phone:     strings.TrimSpace(req.Phone),
		Region:    strings.TrimSpace(req.Region),
		Detail:    strings.TrimSpace(req.Detail),
		IsDefault: req.IsDefault,
	})
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, addr)
}

// PUT /api/v1/addresses/{id} 更新地址
func (a *API) updateAddress(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	id := r.PathValue("id")
	var req addressReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if msg := validAddressReq(req); msg != "" {
		fail(w, http.StatusBadRequest, msg)
		return
	}
	addr, err := a.store.SaveAddress(u.ID, store.Address{
		ID:        id,
		Name:      strings.TrimSpace(req.Name),
		Phone:     strings.TrimSpace(req.Phone),
		Region:    strings.TrimSpace(req.Region),
		Detail:    strings.TrimSpace(req.Detail),
		IsDefault: req.IsDefault,
	})
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, addr)
}

// DELETE /api/v1/addresses/{id} 删除地址
func (a *API) deleteAddress(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if err := a.store.DeleteAddress(u.ID, r.PathValue("id")); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, map[string]string{"id": r.PathValue("id")})
}

// POST /api/v1/addresses/{id}/default 设为默认地址
func (a *API) setDefaultAddress(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if err := a.store.SetDefaultAddress(u.ID, r.PathValue("id")); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, map[string]string{"id": r.PathValue("id")})
}

// ==================== 订单 ====================

type createOrderReq struct {
	AddressID string                   `json:"addressId"`
	Remark    string                   `json:"remark"`
	Items     []store.OrderItemInput   `json:"items"`
}

// POST /api/v1/orders 创建订单（服务器按商品表价格计价，存地址/商品快照）
func (a *API) createOrder(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	var req createOrderReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if strings.TrimSpace(req.AddressID) == "" {
		fail(w, http.StatusBadRequest, "请选择收货地址")
		return
	}
	if len(req.Items) == 0 {
		fail(w, http.StatusBadRequest, "订单商品不能为空")
		return
	}
	if len(req.Items) > 50 {
		fail(w, http.StatusBadRequest, "订单商品种类过多（最多 50 种）")
		return
	}
	for _, it := range req.Items {
		if strings.TrimSpace(it.ProductID) == "" {
			fail(w, http.StatusBadRequest, "订单商品数据无效")
			return
		}
		if it.Quantity <= 0 || it.Quantity > 999 {
			fail(w, http.StatusBadRequest, "商品数量须在 1 ~ 999 之间")
			return
		}
	}
	if tooLong(req.Remark, maxRemark) {
		fail(w, http.StatusBadRequest, "订单备注最长 200 个字符")
		return
	}
	order, err := a.store.CreateOrder(u.ID, req.AddressID, req.Remark, req.Items)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, order)
}

// GET /api/v1/orders?status=all|pending|paid|shipped|done|canceled 我的订单
func (a *API) listOrders(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	status := r.URL.Query().Get("status")
	list, err := a.store.ListOrders(u.ID, status)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, list)
}

// GET /api/v1/orders/{id} 订单详情
func (a *API) getOrder(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	order, err := a.store.GetOrder(u.ID, r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if order == nil {
		fail(w, http.StatusNotFound, "订单不存在")
		return
	}
	ok(w, order)
}

// POST /api/v1/orders/{id}/cancel 取消订单（仅待付款可取消）
func (a *API) cancelOrder(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	order, err := a.store.GetOrder(u.ID, r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if order == nil {
		fail(w, http.StatusNotFound, "订单不存在")
		return
	}
	if order.Status != "pending" {
		fail(w, http.StatusBadRequest, "当前状态不可取消")
		return
	}
	updated, err := a.store.UpdateOrderStatus(u.ID, order.ID, "canceled")
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, updated)
}

// POST /api/v1/orders/{id}/pay 支付订单
// 架构：小程序 → 服务器 → 微信支付（JSAPI 统一下单）
//   - 真实模式（已配置商户凭据）：服务器调微信统一下单 → 返回 wx.requestPayment 参数
//     → 小程序拉起支付 → 微信回调 /api/v1/pay/notify 确认订单为 paid
//   - 模拟模式（未配置）：服务器直接确认订单已支付，便于演示
func (a *API) payOrder(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	var req struct {
		PayMethod string `json:"payMethod"` // wechat / alipay / bank
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if req.PayMethod == "" {
		req.PayMethod = "wechat"
	}
	switch req.PayMethod {
	case "wechat", "alipay", "bank":
	default:
		fail(w, http.StatusBadRequest, "不支持的支付方式")
		return
	}
	order, err := a.store.GetOrder(u.ID, r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if order == nil {
		fail(w, http.StatusNotFound, "订单不存在")
		return
	}
	if order.Status == "paid" || order.Status == "shipped" || order.Status == "done" {
		ok(w, map[string]interface{}{"order": order, "mode": "already"})
		return
	}
	if order.Status != "pending" {
		fail(w, http.StatusBadRequest, "当前订单状态不可支付")
		return
	}

	// ===== 真实微信支付（JSAPI 统一下单）=====
	if a.wpay.Configured() {
		if u.OpenID == "" {
			fail(w, http.StatusBadRequest, "请先在小程序内完成微信授权（wx-login 绑定 openid）后再支付")
			return
		}
		fen := int64(math.Round(order.Total * 100))
		if fen <= 0 {
			fail(w, http.StatusBadRequest, "订单金额无效")
			return
		}
		prepayID, err := a.wpay.JSAPIPrepay(u.OpenID, order.OrderNo, "安全监护商城-"+order.OrderNo, fen)
		if err != nil {
			fail(w, http.StatusBadGateway, "微信统一下单失败: "+err.Error())
			return
		}
		params := a.wpay.PaymentParams(prepayID)
		params["orderId"] = order.ID
		params["orderNo"] = order.OrderNo
		params["mode"] = "real"
		ok(w, params)
		return
	}

	// ===== 模拟支付（未配置微信支付凭据）=====
	order, err = a.store.MarkOrderPaid(u.ID, order.ID, req.PayMethod, "")
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	a.accumulateTeamBusiness(order)
	// 被邀请人支付成功后：生成待结算佣金（无理由退货期满后到账）
	if err := a.store.CreateOrderCommission(order); err != nil {
		log.Printf("[commission] 生成订单佣金失败: %v", err)
	}
	ok(w, map[string]interface{}{
		"order": order,
		"mode":  "simulate",
		"hint":  "未配置微信支付凭据，本次为模拟支付；配置 WECHAT_PAY_MCH_ID/MCH_KEY/SERIAL_NO/PRIVATE_KEY_PATH/PUBLIC_KEY_PATH 后自动走真实微信支付",
	})
}

// accumulateTeamBusiness 支付成功后：服务商品按来源团队累计经营金额（建团资格）
// 并按「平台抽成 10% + 团队金库 90%」规则分成入账
func (a *API) accumulateTeamBusiness(order *store.Order) {
	for _, it := range order.Items {
		if it.Service && strings.TrimSpace(it.SourceTeam) != "" {
			_ = a.store.AddTeamServiceRevenueByName(it.SourceTeam, it.Price*float64(it.Count))
		}
	}
}

// POST /api/v1/pay/notify 微信支付结果回调（微信服务器直连）
// 验签 + AES-GCM 解密 → 按 out_trade_no（商户订单号）确认订单已支付。
// 微信要求返回 {"code":"SUCCESS","message":"成功"} 格式。
func (a *API) payNotify(w http.ResponseWriter, r *http.Request) {
	if !a.wpay.Configured() {
		fail(w, http.StatusBadRequest, "未启用微信支付")
		return
	}
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		fail(w, http.StatusBadRequest, "读取回调内容失败")
		return
	}
	payload, err := a.wpay.VerifyAndDecryptNotify(
		r.Header.Get("Wechatpay-Timestamp"),
		r.Header.Get("Wechatpay-Nonce"),
		r.Header.Get("Wechatpay-Signature"),
		string(bodyBytes),
	)
	if err != nil {
		log.Printf("[pay] 回调验签/解密失败: %v", err)
		fail(w, http.StatusBadRequest, "回调校验失败")
		return
	}
	if payload.TradeState != "SUCCESS" {
		log.Printf("[pay] 回调非成功状态 trade_state=%s，忽略", payload.TradeState)
		w.Write([]byte(`{"code":"SUCCESS","message":"成功"}`))
		return
	}
	order, err := a.store.MarkOrderPaidByOrderNo(payload.OutTradeNo, "wechat", payload.TransactionID)
	if err != nil {
		log.Printf("[pay] 回调更新订单失败: %v", err)
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	a.accumulateTeamBusiness(order)
	// 被邀请人支付成功后：生成待结算佣金（无理由退货期满后到账）
	if err := a.store.CreateOrderCommission(order); err != nil {
		log.Printf("[commission] 生成订单佣金失败: %v", err)
	}
	log.Printf("[pay] 订单 %s 支付成功（微信交易号 %s）", payload.OutTradeNo, payload.TransactionID)
	w.Write([]byte(`{"code":"SUCCESS","message":"成功"}`))
}

// POST /api/v1/orders/{id}/refund 无理由退款（支付后无理由退货期内可退）
// 退款立即生效（订单 refunded），并取消该订单关联的待结算佣金，杜绝退款不回收佣金
func (a *API) refundOrder(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	order, err := a.store.RefundOrder(u.ID, r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	a.store.CancelOrderCommissions(order.ID)
	ok(w, map[string]interface{}{
		"order": order,
		"hint":  fmt.Sprintf("退款成功，订单关联的待结算佣金已取消（无理由退货期 %d 天）", store.CommissionSettleDays()),
	})
}

// POST /api/v1/orders/{id}/confirm 确认收货（shipped → done）
func (a *API) confirmOrder(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	order, err := a.store.GetOrder(u.ID, r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if order == nil {
		fail(w, http.StatusNotFound, "订单不存在")
		return
	}
	if order.Status != "shipped" {
		fail(w, http.StatusBadRequest, "当前状态不可确认收货")
		return
	}
	updated, err := a.store.UpdateOrderStatus(u.ID, order.ID, "done")
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, updated)
}


// ==================== 提现 ====================

type withdrawReq struct {
	Amount  float64 `json:"amount"`
	Method  string  `json:"method"` // wechat / alipay / bank
	Account string  `json:"account"`
}

// POST /api/v1/withdrawals 申请提现
// 服务器事务内扣减余额。
//   - 真实微信支付模式：仅支持提现到微信零钱，收款账户为绑定的微信 openid（商家转账）
//   - 模拟模式：收款账户为客户端填写账号，企业银行卡号（.env 配置）由服务器写入提现记录
func (a *API) applyWithdraw(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	var req withdrawReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if req.Amount <= 0 || req.Amount > 100000 {
		fail(w, http.StatusBadRequest, "提现金额无效（单笔上限 10 万元）")
		return
	}
	if req.Method == "" {
		req.Method = "wechat"
	}
	switch req.Method {
	case "wechat", "alipay", "bank":
	default:
		fail(w, http.StatusBadRequest, "不支持的提现方式")
		return
	}
	// 真实模式：仅支持微信零钱提现（微信支付接口），收款账户为绑定 openid
	account := req.Account
	mode := "simulate"
	if a.wpay.Configured() {
		mode = "real"
		if req.Method != "wechat" {
			fail(w, http.StatusBadRequest, "当前已启用微信支付，仅支持提现到微信零钱")
			return
		}
		if u.OpenID == "" {
			fail(w, http.StatusBadRequest, "请先在小程序内完成微信授权（wx-login 绑定 openid）后再提现")
			return
		}
		account = u.OpenID
	}
	if tooLong(account, maxAccount) {
		fail(w, http.StatusBadRequest, "提现账号最长 50 个字符")
		return
	}
	rec, err := a.store.ApplyWithdrawal(u.ID, req.Method, account, req.Amount, 0, a.pay.BankCardNo)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, map[string]interface{}{
		"withdrawal": rec,
		"mode":       mode, // real = 后台审核通过时经微信转账到零钱；simulate = 演示确认
		"bankCardNo": a.pay.BankCardNo,
	})
}

// GET /api/v1/withdrawals 我的提现记录
func (a *API) listWithdrawals(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	list, err := a.store.ListWithdrawals(u.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, list)
}

// ==================== 管理端：订单 / 提现 ====================

// GET /api/v1/admin/orders?status= 全部订单（发货管理）
func (a *API) adminListOrders(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	list, err := a.store.ListAllOrders(status)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, list)
}

type shipReq struct {
	Company string `json:"company"` // 物流公司
	ShipNo  string `json:"shipNo"`  // 物流单号
}

// POST /api/v1/admin/orders/{id}/ship 后台发货（绑定物流公司与单号）
func (a *API) adminShipOrder(w http.ResponseWriter, r *http.Request) {
	var req shipReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if strings.TrimSpace(req.Company) == "" || strings.TrimSpace(req.ShipNo) == "" {
		fail(w, http.StatusBadRequest, "请填写物流公司与物流单号")
		return
	}
	if tooLong(req.Company, maxCompany) || tooLong(req.ShipNo, maxShipNo) {
		fail(w, http.StatusBadRequest, "物流公司与单号长度不正确")
		return
	}
	order, err := a.store.UpdateOrderShip(r.PathValue("id"), req.Company, req.ShipNo)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, order)
}

// GET /api/v1/admin/withdrawals 全部提现申请（审核）
func (a *API) adminListWithdrawals(w http.ResponseWriter, r *http.Request) {
	list, err := a.store.ListAllWithdrawals()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, list)
}

// POST /api/v1/admin/withdrawals/{id}/complete 提现打款完成
//   - 真实微信支付模式：调用微信「商家转账到零钱」接口向用户 openid 打款
//   - 模拟模式：直接标记打款完成（演示）
func (a *API) adminCompleteWithdrawal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rec, err := a.store.GetWithdrawal(id)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rec == nil {
		fail(w, http.StatusNotFound, "提现申请不存在")
		return
	}
	if rec.Status != "processing" {
		fail(w, http.StatusBadRequest, "该申请已处理")
		return
	}
	// ===== 真实模式：微信转账到零钱 =====
	if a.wpay.Configured() {
		if rec.Method != "wechat" || rec.Account == "" {
			fail(w, http.StatusBadRequest, "真实支付模式仅支持微信零钱提现，该申请无法通过微信转账打款")
			return
		}
		if rec.Amount < 0.3 {
			fail(w, http.StatusBadRequest, "微信转账单笔最低 0.3 元")
			return
		}
		fen := int64(math.Round(rec.Amount * 100))
		batchID, err := a.wpay.TransferToBalance(rec.Account, "TX"+rec.ID, "TX"+rec.ID, "商城提现", fen)
		if err != nil {
			log.Printf("[pay] 提现 %s 微信转账失败: %v", id, err)
			fail(w, http.StatusBadGateway, "微信转账失败: "+err.Error())
			return
		}
		done, err := a.store.CompleteWithdrawal(id, "done")
		if err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		log.Printf("[pay] 提现 %s 已通过微信转账（批次号 %s）", id, batchID)
		ok(w, map[string]interface{}{"withdrawal": done, "mode": "real", "batchId": batchID})
		return
	}
	// ===== 模拟模式 =====
	done, err := a.store.CompleteWithdrawal(id, "done")
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, map[string]interface{}{"withdrawal": done, "mode": "simulate"})
}

// POST /api/v1/admin/withdrawals/{id}/fail 驳回提现（自动退款到余额）
func (a *API) adminFailWithdrawal(w http.ResponseWriter, r *http.Request) {
	rec, err := a.store.CompleteWithdrawal(r.PathValue("id"), "failed")
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, rec)
}

