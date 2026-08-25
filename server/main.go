// main.go
// 安全监护商城 · Go 服务端入口
//
// 启动方式：
//
//	go run main.go        # 普通模式
//	go run main.go -debug # DEBUG 模式（输出请求日志）
//	DEBUG=1 go run main.go
//
// 环境变量：
//
//	ADDR  监听地址（默认 :8080）
//	DEBUG 1 / true 时开启 debug 模式
//	WECHAT_APPID       微信小程序 AppID（企业主体）
//	WECHAT_SECRET      微信小程序 AppSecret
//	WECHAT_TEMPLATE_ID 订阅消息模板 ID（微信公众平台申请）
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"socialserver/internal/api"
	"socialserver/internal/hwplatform"
	"socialserver/internal/store"
	"socialserver/internal/wechat"
)

// loadEnvFile 加载 .env 文件（KEY=VALUE，支持 export 前缀与 # 注释）。
// 已存在的环境变量优先，不会被 .env 覆盖。
func loadEnvFile(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.TrimSpace(kv[0])
		v := strings.Trim(strings.TrimSpace(kv[1]), `"'`)
		if k == "" {
			continue
		}
		if _, ok := os.LookupEnv(k); !ok {
			os.Setenv(k, v)
		}
	}
}

func main() {
	debugFlag := flag.Bool("debug", false, "启用 debug 模式（输出请求日志）")
	flag.Parse()

	// 加载 .env（含微信支付企业银行卡号等配置；环境变量优先）
	// 兼容两种启动方式：cd server && go run main.go 或 根目录 go run ./server/main.go
	for _, envPath := range []string{".env", "server/.env"} {
		loadEnvFile(envPath)
	}

	s := store.New()

	// 佣金结算等待天数（无理由退货期）：订单支付后 x 天佣金才到账，期内退款则取消
	if d := os.Getenv("COMMISSION_SETTLE_DAYS"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 {
			store.SetCommissionSettleDays(n)
		}
	}
	log.Printf("[commission] 无理由退货期 = %d 天（订单支付后佣金延迟到账；期内退款佣金取消，可用 COMMISSION_SETTLE_DAYS 调整）", store.CommissionSettleDays())
	wc := wechat.New(wechat.Config{
		AppID:      os.Getenv("WECHAT_APPID"),
		AppSecret:  os.Getenv("WECHAT_SECRET"),
		TemplateID: os.Getenv("WECHAT_TEMPLATE_ID"),
	})

	// 硬件开放平台（健康/养老设备）：绑定校验 + 平台推送接收
	// 未配置 HARDWARE_APPID/HARDWARE_APPKEY 时进入模拟模式（UID 按 IMEI 派生），
	// 便于本地联调「设备绑定 → 平台推送 → 消息路由 → 微信通知」整条链路。
	hwc := hwplatform.New(hwplatform.Config{
		AppID:   os.Getenv("HARDWARE_APPID"),
		AppKey:  os.Getenv("HARDWARE_APPKEY"),
		BaseURL: os.Getenv("HARDWARE_API_BASE"),
	})

	// 初始化 SQLite（商品审计等持久化；DB_PATH 可自定义）
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/app.db"
	}
	if err := s.OpenDB(dbPath); err != nil {
		log.Printf("[db] ⚠️ SQLite 连接失败: %v", err)
	} else {
		log.Printf("[db] ✅ SQLite 已连接: %s", dbPath)
	}

	// 商品图片上传目录
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "uploads"
	}

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	debug := *debugFlag || os.Getenv("DEBUG") == "1" || os.Getenv("DEBUG") == "true"

	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		adminPassword = "admin123"
		log.Printf("[admin] 管理员密码使用默认值 admin123（可用环境变量 ADMIN_PASSWORD 修改）")
	}
	adminUsername := os.Getenv("ADMIN_USERNAME")
	if adminUsername == "" {
		adminUsername = "admin"
	}
	// 初始化超级管理员账号（仅首次自动创建）
	if err := s.InitAdmin(adminUsername, adminPassword); err != nil {
		log.Printf("[admin] ⚠️ 初始化 admin 账号失败: %v", err)
	} else {
		log.Printf("[admin] ✅ 管理端账号已就绪：%s（密码可通过 ADMIN_PASSWORD 修改）", adminUsername)
	}

	var handler http.Handler = api.New(s, wc, hwc, adminPassword, uploadDir, api.PayConfig{
		MchID:      os.Getenv("WECHAT_PAY_MCH_ID"),             // 微信支付商户号
		MchKey:     os.Getenv("WECHAT_PAY_MCH_KEY"),            // 微信支付 APIv3 密钥（32 位）
		SerialNo:   os.Getenv("WECHAT_PAY_SERIAL_NO"),          // 商户 API 证书序列号
		PrivateKey: os.Getenv("WECHAT_PAY_PRIVATE_KEY_PATH"),   // 商户 API 私钥 apiclient_key.pem 路径
		PublicKey:  os.Getenv("WECHAT_PAY_PUBLIC_KEY_PATH"),    // 微信支付公钥 PEM 路径
		AppID:      os.Getenv("WECHAT_APPID"),                  // 小程序 AppID
		BankCardNo: os.Getenv("WECHAT_PAY_BANK_CARD"),          // 企业银行卡号（演示模式提现打款收款账户）
		NotifyURL:  os.Getenv("WECHAT_PAY_NOTIFY_URL"),         // 支付结果回调地址
	})
	if debug {
		handler = api.WithDebugLog(handler)
		log.Printf("[debug] DEBUG 模式已开启")
	}

	if wc.Configured() {
		log.Printf("[wechat] 订阅消息推送已启用（模板 ID: %s）", wc.TemplateID())
	} else {
		log.Printf("[wechat] 未配置订阅消息凭据，推送将使用模拟模式")
		log.Printf("[wechat] 启用真实推送：export WECHAT_APPID=... WECHAT_SECRET=... WECHAT_TEMPLATE_ID=...")
	}

	if hwc.Configured() {
		log.Printf("[hw] ✅ 硬件开放平台已配置（appId: %s, %s），设备绑定将实时校验IMEI", hwc.AppID(), hwc.BaseURL())
	} else {
		log.Printf("[hw] 未配置硬件开放平台凭据（HARDWARE_APPID/APPKEY），使用模拟模式（UID 按 IMEI 派生）")
		log.Printf("[hw] 平台推送接收地址：POST /api/v1/hw/push（在平台「个人中心→数据上传url」配置）")
	}

	mchID := os.Getenv("WECHAT_PAY_MCH_ID")
	payReady := os.Getenv("WECHAT_PAY_MCH_ID") != "" &&
		os.Getenv("WECHAT_PAY_MCH_KEY") != "" &&
		os.Getenv("WECHAT_PAY_SERIAL_NO") != "" &&
		os.Getenv("WECHAT_PAY_PRIVATE_KEY_PATH") != "" &&
		os.Getenv("WECHAT_PAY_PUBLIC_KEY_PATH") != ""
	if payReady {
		log.Printf("[pay] ✅ 微信支付已启用（商户号: %s），支付/提现走真实微信支付 APIv3", mchID)
	} else {
		log.Printf("[pay] ⚠️ 未配置完整微信支付凭据（WECHAT_PAY_MCH_ID/MCH_KEY/SERIAL_NO/PRIVATE_KEY_PATH/PUBLIC_KEY_PATH），支付/提现使用模拟模式")
		log.Printf("[pay]   启用真实微信支付：配置上述环境变量 + WECHAT_PAY_NOTIFY_URL（公网 HTTPS 回调地址）")
	}
	if bankCard := os.Getenv("WECHAT_PAY_BANK_CARD"); bankCard != "" {
		log.Printf("[pay] 企业银行卡号已配置（%s），模拟模式提现打款将绑定该卡在服务器处理", bankCard)
	}

	log.Printf("安全监护商城服务端已启动: http://localhost%s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}
