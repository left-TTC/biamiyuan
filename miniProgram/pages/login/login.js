// pages/login/login.js
const store = require('../../utils/store.js')
const api = require('../../utils/api.js')

Page({
    data: {
        mode: 'login', // login 登录 / register 注册
        phone: '',
        code: '',
        smsCode: store.SMS_CODE,
        counting: false,
        countdown: 60,
        agree: true,
        submitting: false,
    },

    onLoad() {
        if (store.isLoggedIn()) {
            wx.switchTab({ url: '/pages/index/index' })
        }
    },

    onUnload() {
        if (this.timer) clearInterval(this.timer)
    },

    onPhone(e) {
        this.setData({ phone: e.detail.value })
    },

    switchMode(e) {
        this.setData({ mode: e.currentTarget.dataset.mode })
    },

    // 手机号快捷填充回调（phone-number-quickfill 官方组件）
    onGetPhoneNumber(e) {
        const detail = e.detail || {}
        // 用户取消填充
        if (detail.errMsg && detail.errMsg.indexOf('cancel') > -1) return
        // 真实环境：detail.code 发送服务端，经 code2Session 换取微信绑定手机号后填入
        // 演示环境：模拟填入微信手机号
        wx.showToast({ title: '已填入微信手机号', icon: 'none' })
        this.setData({ phone: '13800138000' })
    },

    onCode(e) {
        this.setData({ code: e.detail.value })
    },

    toggleAgree() {
        this.setData({ agree: !this.data.agree })
    },

    // 获取验证码：通过服务器发送（演示接口，注册/登录现已不强制校验验证码）
    sendCode() {
        if (this.data.counting || this.data.submitting) return
        const phone = this.data.phone.trim()
        if (!phone) {
            wx.showToast({ title: '请先输入手机号', icon: 'none' })
            return
        }
        wx.showLoading({ title: '发送中…' })
        api.request('/api/v1/auth/code', { method: 'POST', data: { phone }, noRetry: true })
            .then(() => {
                wx.hideLoading()
                wx.showToast({ title: '验证码已发送（演示为 ' + this.data.smsCode + '）', icon: 'none' })
                this.setData({ counting: true, countdown: 60 })
                this.timer = setInterval(() => {
                    const n = this.data.countdown - 1
                    if (n <= 0) {
                        clearInterval(this.timer)
                        this.timer = null
                        this.setData({ counting: false, countdown: 60 })
                    } else {
                        this.setData({ countdown: n })
                    }
                }, 1000)
            })
            .catch((err) => {
                wx.hideLoading()
                wx.showToast({ title: (err && err.message) || '验证码发送失败', icon: 'none' })
            })
    },

    // 一键填入演示账号
    fillDemo() {
        this.setData({ phone: 'test', code: this.data.smsCode })
    },

    // 登录 / 注册：均以服务器为准，未注册的账号必须先注册
    // 演示环境：验证码怎么填都通过（可不填），手机号需非空
    submit() {
        const phone = this.data.phone.trim()
        const code = this.data.code.trim()
        if (!this.data.agree) {
            wx.showToast({ title: '请先阅读并同意协议', icon: 'none' })
            return
        }
        if (!phone) {
            wx.showToast({ title: '请输入手机号', icon: 'none' })
            return
        }
        if (this.data.submitting) return
        this.setData({ submitting: true })
        wx.showLoading({ title: '处理中…' })

        const isRegister = this.data.mode === 'register'
        const action = isRegister
            ? store.registerWithPhone(phone, code)
            : store.loginWithPhone(phone, code)

        action
            .then(() => {
                wx.hideLoading()
                wx.showToast({ title: isRegister ? '注册成功' : '登录成功', icon: 'success' })
                setTimeout(() => {
                    // 返回发起登录的页面，继续之前的操作（如加入购物车后回来）
                    wx.navigateBack({
                        fail: () => wx.switchTab({ url: '/pages/index/index' }),
                    })
                }, 600)
            })
            .catch((err) => {
                wx.hideLoading()
                const msg = (err && err.message) || '操作失败，请重试'
                if (msg.indexOf('已注册') > -1) {
                    wx.showModal({
                        title: '提示',
                        content: msg + '，请切换到「登录」',
                        showCancel: false,
                        success: () => this.setData({ mode: 'login' }),
                    })
                } else if (msg.indexOf('未注册') > -1) {
                    wx.showModal({
                        title: '提示',
                        content: msg + '，请切换到「注册」',
                        showCancel: false,
                        success: () => this.setData({ mode: 'register' }),
                    })
                } else {
                    wx.showToast({ title: msg, icon: 'none' })
                }
            })
            .finally(() => this.setData({ submitting: false }))
    },
})
