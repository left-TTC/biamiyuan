// validate.go
// 输入校验辅助函数：统一处理字符串长度（按字符计）、手机号格式等基础审核。
// 各 handler 在 Decode 后调用，保证入库/入内存的数据长度与数值在合理范围内。
package api

import (
	"strings"
	"unicode/utf8"
)

// runeLen 按字符（非字节）计数，中文 / emoji / 4 字节字符均按 1 计
func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}

// tooLong 判断字符串是否超过 n 个字符
func tooLong(s string, n int) bool {
	return runeLen(s) > n
}

// validPhone 校验中国大陆手机号（11 位，1 开头）
func validPhone(p string) bool {
	return phoneRe.MatchString(strings.TrimSpace(p))
}

// validSupportContent 校验客服消息内容，返回裁剪后的内容与错误信息（空串表示通过）
func validSupportContent(content string) (string, string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", "消息内容不能为空"
	}
	if tooLong(content, maxMsgContent) {
		return "", "消息内容最长 1000 个字符"
	}
	return content, ""
}

// 常用字段长度上限（字符数）
const (
	maxNickName    = 20  // 昵称
	maxTeamName    = 20  // 团队名称
	maxProductName = 60  // 商品 / 服务名称
	maxDesc        = 200 // 商品 / 服务描述
	maxAddressName = 20  // 收货人
	maxRegion      = 60  // 所在地区
	maxAddressDet  = 120 // 详细地址
	maxRemark      = 200 // 订单备注
	maxSubject     = 50  // 客服问题标题
	maxMsgContent  = 1000 // 客服消息 / 问题描述
	maxDeviceName  = 30  // 设备名称
	maxSN          = 50  // 设备序列号
	maxAccount     = 50  // 提现账号
	maxCompany     = 30  // 物流公司
	maxShipNo      = 50  // 物流单号
	maxUsername    = 32  // 管理端用户名
	maxPassword    = 64  // 管理端密码
	maxTeamID      = 40  // 团队 ID
)
