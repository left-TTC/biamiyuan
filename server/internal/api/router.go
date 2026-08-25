// router.go
// HTTP 路由注册
package api

import (
	"net/http"

	"socialserver/internal/hwplatform"
	"socialserver/internal/store"
	"socialserver/internal/wechat"
	"socialserver/internal/wechatpay"
)

// API HTTP 处理器
type API struct {
	store         *store.Store
	wechat        *wechat.Client // 可为 nil（未配置微信凭据时）
	hw            *hwplatform.Client // 硬件开放平台客户端（未配置凭据时进入模拟模式）
	adminPassword string
	uploadDir     string // 商品图片存储目录
	pay           PayConfig // 微信支付/提现配置（.env）
	wpay          *wechatpay.Client // 微信支付 APIv3 客户端（未配置时模拟模式）
}

// PayConfig 支付与提现配置（从 .env 读取，服务器统一处理）
type PayConfig struct {
	MchID      string // 微信支付商户号
	MchKey     string // 微信支付 APIv3 密钥（32 位）
	SerialNo   string // 商户 API 证书序列号
	PrivateKey string // 商户 API 私钥（apiclient_key.pem 路径）
	PublicKey  string // 微信支付公钥（PEM 路径，转账加密 + 回调验签）
	AppID      string // 小程序 AppID（复用 WECHAT_APPID）
	BankCardNo string // 企业银行卡号（演示模式提现打款收款卡）
	NotifyURL  string // 支付结果回调地址
}

// New 构建路由
func New(s *store.Store, wc *wechat.Client, hwc *hwplatform.Client, adminPassword, uploadDir string, pay PayConfig) http.Handler {
	a := &API{
		store:         s,
		wechat:        wc,
		hw:            hwc,
		adminPassword: adminPassword,
		uploadDir:     uploadDir,
		pay:           pay,
		wpay: wechatpay.New(wechatpay.Config{
			AppID:      pay.AppID,
			MchID:      pay.MchID,
			APIv3Key:   pay.MchKey,
			SerialNo:   pay.SerialNo,
			PrivateKey: pay.PrivateKey,
			PublicKey:  pay.PublicKey,
			NotifyURL:  pay.NotifyURL,
		}),
	}
	mux := http.NewServeMux()

	// 商品图片静态文件服务（/uploads/img_xxx.jpg）
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadDir))))

	// 健康检查
	mux.HandleFunc("GET /health", a.health)

	// 微信支付结果回调（微信服务器直连，无需登录鉴权；服务器验签后确认订单已支付）
	mux.HandleFunc("POST /api/v1/pay/notify", a.payNotify)

	// 认证
	mux.HandleFunc("POST /api/v1/auth/code", a.sendCode)
	mux.HandleFunc("POST /api/v1/auth/register", a.register)
	mux.HandleFunc("POST /api/v1/auth/login", a.login)
	mux.HandleFunc("POST /api/v1/auth/wx-login", a.auth(a.wxLogin))

	// 用户（需登录）
	mux.HandleFunc("GET /api/v1/user/me", a.auth(a.userMe))
	mux.HandleFunc("PUT /api/v1/user/profile", a.auth(a.updateProfile))
	mux.HandleFunc("POST /api/v1/user/avatar", a.auth(a.uploadAvatar))
	mux.HandleFunc("POST /api/v1/user/bind-inviter", a.auth(a.bindInviter))
	mux.HandleFunc("GET /api/v1/user/invitees", a.auth(a.myInvitees))
	mux.HandleFunc("GET /api/v1/user/commissions", a.auth(a.myCommissions))
	// 单机演示：模拟好友下单产生延迟到账佣金
	mux.HandleFunc("POST /api/v1/user/commission/demo", a.auth(a.demoCommission))

	// 购物车历史（需登录，SQLite 持久化，记录商品 id 与数量）
	mux.HandleFunc("GET /api/v1/cart", a.auth(a.getCart))
	mux.HandleFunc("POST /api/v1/cart/sync", a.auth(a.syncCart))
	mux.HandleFunc("PUT /api/v1/cart/items/{productId}", a.auth(a.updateCartItem))
	mux.HandleFunc("DELETE /api/v1/cart/items/{productId}", a.auth(a.deleteCartItem))

	// 团队（需登录；建团资格：邀请 >2 人或所在团队经营金额 >1w）
	mux.HandleFunc("GET /api/v1/team/my", a.auth(a.myTeam))
	mux.HandleFunc("POST /api/v1/team/create", a.auth(a.createTeam))
	mux.HandleFunc("POST /api/v1/team/join", a.auth(a.joinTeam))
	mux.HandleFunc("POST /api/v1/team/business", a.auth(a.addTeamBusiness))
	mux.HandleFunc("POST /api/v1/team/products", a.auth(a.teamPublishService))
	// 团员建新团申请 + 团长审核
	mux.HandleFunc("POST /api/v1/team/apply-create", a.auth(a.applyCreateTeam))
	mux.HandleFunc("GET /api/v1/team/requests/my", a.auth(a.myTeamRequests))
	mux.HandleFunc("GET /api/v1/team/requests/inbox", a.auth(a.teamRequestInbox))
	mux.HandleFunc("POST /api/v1/team/requests/{id}/approve", a.auth(a.approveTeamRequest))
	mux.HandleFunc("POST /api/v1/team/requests/{id}/reject", a.auth(a.rejectTeamRequest))
	// 团队入团邀请（团长邀请 → 对方同意后入团）
	mux.HandleFunc("GET /api/v1/team/invites/candidates", a.auth(a.teamInviteCandidates))
	mux.HandleFunc("POST /api/v1/team/invites", a.auth(a.createTeamInvite))
	mux.HandleFunc("GET /api/v1/team/invites/outbox", a.auth(a.teamInviteOutbox))
	mux.HandleFunc("GET /api/v1/team/invites/inbox", a.auth(a.teamInviteInbox))
	mux.HandleFunc("POST /api/v1/team/invites/{id}/accept", a.auth(a.acceptTeamInvite))
	mux.HandleFunc("POST /api/v1/team/invites/{id}/reject", a.auth(a.rejectTeamInvite))
	mux.HandleFunc("POST /api/v1/team/invites/{id}/cancel", a.auth(a.cancelTeamInvite))
	// 手机号查询用户（邀请入团）
	mux.HandleFunc("GET /api/v1/users/search", a.auth(a.searchUser))

	// 商品（公开）
	mux.HandleFunc("GET /api/v1/categories", a.listCategories)
	mux.HandleFunc("GET /api/v1/products", a.listProducts)
	mux.HandleFunc("GET /api/v1/products/{id}", a.getProduct)

	// 设备（需登录）
	mux.HandleFunc("GET /api/v1/devices", a.auth(a.listDevices))
	mux.HandleFunc("POST /api/v1/devices", a.auth(a.addDevice))
	mux.HandleFunc("DELETE /api/v1/devices/{id}", a.auth(a.removeDevice))
	mux.HandleFunc("POST /api/v1/devices/{id}/alarm", a.auth(a.alarmDevice))
	mux.HandleFunc("POST /api/v1/devices/alarm-all", a.auth(a.alarmAll))
	// 硬件开放平台：绑定设备（校验IMEI/获取UID，UID全局唯一保证一台硬件只归属一个用户）
	mux.HandleFunc("POST /api/v1/devices/bind", a.auth(a.bindDevice))
	// 硬件开放平台：更新设备名称/通知开关
	mux.HandleFunc("PUT /api/v1/devices/{id}", a.auth(a.updateDevice))
	// 硬件开放平台：数据推送接收（webhook，平台「个人中心→数据上传url」填该地址，无需登录）
	mux.HandleFunc("POST /api/v1/hw/push", a.hwPush)
	// 硬件开放平台：推送模拟（本地联调「推送→消息→微信通知」全链路）
	mux.HandleFunc("POST /api/v1/hw/simulate", a.auth(a.simulateHwPush))
	// 硬件开放平台：对接状态查询
	mux.HandleFunc("GET /api/v1/hw/status", a.auth(a.hwStatus))

	// 消息（需登录）
	mux.HandleFunc("GET /api/v1/messages", a.auth(a.listMessages))
	mux.HandleFunc("POST /api/v1/messages/{id}/read", a.auth(a.markMessageRead))

	// 收货地址（需登录，服务器保存并绑定账号；默认地址唯一）
	mux.HandleFunc("GET /api/v1/addresses", a.auth(a.listAddresses))
	mux.HandleFunc("POST /api/v1/addresses", a.auth(a.saveAddress))
	mux.HandleFunc("PUT /api/v1/addresses/{id}", a.auth(a.updateAddress))
	mux.HandleFunc("DELETE /api/v1/addresses/{id}", a.auth(a.deleteAddress))
	mux.HandleFunc("POST /api/v1/addresses/{id}/default", a.auth(a.setDefaultAddress))

	// 订单（需登录；服务器计价 + 支付 + 物流）
	mux.HandleFunc("POST /api/v1/orders", a.auth(a.createOrder))
	mux.HandleFunc("GET /api/v1/orders", a.auth(a.listOrders))
	mux.HandleFunc("GET /api/v1/orders/{id}", a.auth(a.getOrder))
	mux.HandleFunc("POST /api/v1/orders/{id}/cancel", a.auth(a.cancelOrder))
	mux.HandleFunc("POST /api/v1/orders/{id}/pay", a.auth(a.payOrder))
	mux.HandleFunc("POST /api/v1/orders/{id}/confirm", a.auth(a.confirmOrder))
	// 无理由退款（支付后无理由退货期内可退；取消关联待结算佣金）
	mux.HandleFunc("POST /api/v1/orders/{id}/refund", a.auth(a.refundOrder))

	// 提现（需登录；企业银行卡号在服务器写入）
	mux.HandleFunc("POST /api/v1/withdrawals", a.auth(a.applyWithdraw))
	mux.HandleFunc("GET /api/v1/withdrawals", a.auth(a.listWithdrawals))

	// 客服（需登录；团队服务→团队指定客服成员，普通商品/官方服务→后台客服）
	mux.HandleFunc("GET /api/v1/support/tickets", a.auth(a.listSupportTickets))
	mux.HandleFunc("POST /api/v1/support/tickets", a.auth(a.createSupportTicket))
	mux.HandleFunc("GET /api/v1/support/tickets/{id}", a.auth(a.getSupportTicket))
	mux.HandleFunc("POST /api/v1/support/tickets/{id}/messages", a.auth(a.sendSupportMessage))
	// 团队客服成员：收件箱 + 回复 + 关闭
	mux.HandleFunc("GET /api/v1/support/inbox", a.auth(a.mySupportInbox))
	mux.HandleFunc("POST /api/v1/support/tickets/{id}/reply", a.auth(a.replySupportTicket))
	mux.HandleFunc("POST /api/v1/support/tickets/{id}/close", a.auth(a.closeSupportTicket))

	// 订阅消息（需登录）
	mux.HandleFunc("GET /api/v1/notify/template-id", a.auth(a.notifyTemplate))
	mux.HandleFunc("POST /api/v1/notify/subscribe", a.auth(a.subscribe))

	// 管理端（工作台，需管理端账号登录）
	mux.HandleFunc("POST /api/v1/admin/login", a.adminLogin)
	mux.HandleFunc("GET /api/v1/admin/stats", a.adminAuth(a.adminStats))
	mux.HandleFunc("GET /api/v1/admin/db-status", a.adminAuth(a.adminDBStatus))
	mux.HandleFunc("GET /api/v1/admin/users", a.adminAuth(a.adminUsers))
	mux.HandleFunc("GET /api/v1/admin/carts", a.adminAuth(a.adminCarts))
	mux.HandleFunc("GET /api/v1/admin/teams", a.adminAuth(a.adminTeams))
	mux.HandleFunc("DELETE /api/v1/admin/teams/{id}", a.adminAuth(a.adminRemoveTeam))
	// 订单管理（发货/物流绑定）
	mux.HandleFunc("GET /api/v1/admin/orders", a.adminAuth(a.adminListOrders))
	mux.HandleFunc("POST /api/v1/admin/orders/{id}/ship", a.adminAuth(a.adminShipOrder))
	// 提现审核（打款完成/驳回退款）
	mux.HandleFunc("GET /api/v1/admin/withdrawals", a.adminAuth(a.adminListWithdrawals))
	mux.HandleFunc("POST /api/v1/admin/withdrawals/{id}/complete", a.adminAuth(a.adminCompleteWithdrawal))
	mux.HandleFunc("POST /api/v1/admin/withdrawals/{id}/fail", a.adminAuth(a.adminFailWithdrawal))
	// 客服工作台（后台客服收件箱）
	mux.HandleFunc("GET /api/v1/admin/support/tickets", a.adminAuth(a.adminListSupportTickets))
	mux.HandleFunc("GET /api/v1/admin/support/tickets/{id}", a.adminAuth(a.adminGetSupportTicket))
	mux.HandleFunc("POST /api/v1/admin/support/tickets/{id}/reply", a.adminAuth(a.adminReplySupport))
	mux.HandleFunc("POST /api/v1/admin/support/tickets/{id}/close", a.adminAuth(a.adminCloseSupport))
	// 团队客服设置（团长为团队指定客服成员）
	mux.HandleFunc("POST /api/v1/team/support-member", a.auth(a.setTeamSupportMember))
	// 团队金库（服务分成 90% 入金库，仅团长可提取/向成员转账 + 流水）
	mux.HandleFunc("POST /api/v1/team/treasury/withdraw", a.auth(a.treasuryWithdraw))
	mux.HandleFunc("POST /api/v1/team/treasury/transfer", a.auth(a.treasuryTransfer))
	mux.HandleFunc("GET /api/v1/team/treasury/logs", a.auth(a.treasuryLogs))
	mux.HandleFunc("GET /api/v1/admin/devices", a.adminAuth(a.adminDevices))
	mux.HandleFunc("POST /api/v1/admin/devices/{id}/alarm", a.adminAuth(a.adminDeviceAlarm))
	mux.HandleFunc("DELETE /api/v1/admin/devices/{id}", a.adminAuth(a.adminRemoveDevice))
	mux.HandleFunc("GET /api/v1/admin/messages", a.adminAuth(a.adminMessages))
	mux.HandleFunc("DELETE /api/v1/admin/messages/{id}", a.adminAuth(a.adminRemoveMessage))
	// 类目管理
	mux.HandleFunc("GET /api/v1/admin/categories", a.adminAuth(a.adminListCategories))
	mux.HandleFunc("POST /api/v1/admin/categories", a.adminAuth(a.adminCreateCategory))
	mux.HandleFunc("PUT /api/v1/admin/categories/{id}", a.adminAuth(a.adminUpdateCategory))
	mux.HandleFunc("DELETE /api/v1/admin/categories/{id}", a.adminAuth(a.adminDeleteCategory))
	// 商品管理 + 批量导入（Excel 由前端解析为 JSON 提交）
	mux.HandleFunc("GET /api/v1/admin/products", a.adminAuth(a.adminProducts))
	mux.HandleFunc("POST /api/v1/admin/products", a.adminAuth(a.adminCreateProduct))
	mux.HandleFunc("POST /api/v1/admin/products/batch", a.adminAuth(a.adminBatchProducts))
	mux.HandleFunc("POST /api/v1/admin/products/upload", a.adminAuth(a.adminUploadImage))
	mux.HandleFunc("PUT /api/v1/admin/products/{id}", a.adminAuth(a.adminUpdateProduct))
	mux.HandleFunc("DELETE /api/v1/admin/products/{id}", a.adminAuth(a.adminDeleteProduct))

	// 账号管理（仅超级管理员 admin 可访问）
	mux.HandleFunc("GET /api/v1/admin/accounts", a.adminAuth(a.superAdminAuth(a.adminListAccounts)))
	mux.HandleFunc("POST /api/v1/admin/accounts", a.adminAuth(a.superAdminAuth(a.adminCreateAccount)))
	mux.HandleFunc("DELETE /api/v1/admin/accounts/{id}", a.adminAuth(a.superAdminAuth(a.adminDeleteAccount)))
	mux.HandleFunc("PUT /api/v1/admin/accounts/{id}/role", a.adminAuth(a.superAdminAuth(a.adminUpdateAccountRole)))
	mux.HandleFunc("PUT /api/v1/admin/accounts/{id}/status", a.adminAuth(a.superAdminAuth(a.adminUpdateAccountStatus)))

	return mux
}

// GET /health 健康检查
func (a *API) health(w http.ResponseWriter, r *http.Request) {
	ok(w, map[string]interface{}{
		"status": "up",
		"db":     a.store.DBStatus()["connected"],
	})
}
