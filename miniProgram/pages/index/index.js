// pages/index/index.js
const store = require('../../utils/store.js')
const api = require('../../utils/api.js')
const util = require('../../utils/util.js')

Page({
    data: {
        isLogin: false,
        stat: { total: 0, online: 0, alarm: 0, offline: 0 },
        messages: [],
        unread: 0,
        notifyOn: false,
        alarmSending: false,
    },

    onShow() {
        // 同步自定义 tabBar 选中态（首页 = 0）与购物车角标
        if (typeof this.getTabBar === 'function' && this.getTabBar()) {
            this.getTabBar().setData({ selected: 0, cartCount: store.calcCartCount() })
        }
        // 游客可浏览首页与商城；设备相关功能需登录（设备与账号绑定）
        this.refresh()
        this.refreshCartBadge()
    },

    // 统一邀请机制：所有会员均可分享邀请好友（未登录分享不带邀请码）
    onShareAppMessage() {
        const code = store.isLoggedIn() ? store.myCode() : ''
        return {
            title: '安全监护产品商城，邀请你成为会员',
            path: '/pages/index/index' + (code ? '?inviter=' + code : ''),
        }
    },

    refresh() {
        const isLogin = store.isLoggedIn()
        const user = isLogin ? store.getUser() : null
        const render = () => {
            let stat = { total: 0, online: 0, alarm: 0, offline: 0 }
            let messages = []
            let unread = 0
            // 设备与账号绑定：仅登录后读取设备统计与消息
            if (isLogin) {
                messages = store
                    .getDeviceMessages()
                    .slice(0, 8)
                    .map((m) => {
                        const device = store.getDevice(m.deviceId)
                        return Object.assign({}, m, {
                            timeText: util.formatTime(new Date(m.time)),
                            emoji: device ? device.icon : '📟',
                        })
                    })
                stat = store.getDevicesStat()
                unread = store.getUnreadDeviceMsgCount()
            }
            this.setData({
                isLogin,
                stat,
                messages,
                unread,
                notifyOn: isLogin ? !!user.notifyEnabled : false,
            })
        }
        render()
        // 设备/消息以服务器为准（硬件平台推送、报警写入服务器），登录后拉取刷新
        if (isLogin) {
            Promise.all([store.fetchDevices(), store.fetchDeviceMessages()])
                .then(render)
                .catch(() => {})
        }
    },

    goLogin() {
        wx.navigateTo({ url: '/pages/login/login' })
    },

    // 点击设备数量汇总：进入管理设备页（需登录）
    goDevices() {
        if (!store.requireLogin()) return
        wx.navigateTo({ url: '/pages/devices/devices' })
    },

    tapMessage(e) {
        const id = e.currentTarget.dataset.id
        store.markDeviceMsgRead(id)
        const msg = store.getDeviceMessages().find((m) => m.id === id)
        if (msg) {
            wx.navigateTo({ url: '/pages/device/device?id=' + msg.deviceId })
        }
        this.refresh()
    },

    // 开启/关闭报警微信通知（订阅消息）
    // 流程：绑定 openid → 获取模板 ID → wx.requestSubscribeMessage 授权 → 上报订阅
    toggleNotify() {
        if (!store.requireLogin()) return
        const user = store.getUser()
        if (user.notifyEnabled) {
            store.updateUser({ notifyEnabled: false })
            this.setData({ notifyOn: false })
            wx.showToast({ title: '已关闭报警通知', icon: 'none' })
            return
        }
        const self = this

        // 1. 绑定微信 openid
        store
            .bindWxOpenid()
            .then(() => api.request('/api/v1/notify/template-id', { token: store.getServerToken() }))
            .then((res) => {
                const tmplId = res.templateId
                if (!tmplId) {
                    // 服务器未配置订阅模板：演示模式直接开启
                    store.updateUser({ notifyEnabled: true })
                    self.setData({ notifyOn: true })
                    wx.showToast({ title: '已开启报警通知（演示，服务器未配置订阅模板）', icon: 'none' })
                    return
                }
                // 2. 请求订阅授权（一次性订阅）
                wx.requestSubscribeMessage({
                    tmplIds: [tmplId],
                    success: (sr) => {
                        if (sr[tmplId] === 'accept') {
                            // 3. 上报订阅次数
                            api.request('/api/v1/notify/subscribe', {
                                method: 'POST',
                                data: {},
                                token: store.getServerToken(),
                            }).catch(() => {})
                            store.updateUser({ notifyEnabled: true })
                            self.setData({ notifyOn: true })
                            wx.showToast({ title: '已开启报警通知', icon: 'success' })
                        } else {
                            wx.showToast({ title: '未授权订阅，可在设置中重新开启', icon: 'none' })
                        }
                    },
                    fail: () => {
                        // 开发者工具/异常环境：演示模式开启
                        store.updateUser({ notifyEnabled: true })
                        self.setData({ notifyOn: true })
                        wx.showToast({ title: '已开启报警通知（演示）', icon: 'none' })
                    },
                })
            })
            .catch(() => {
                // 服务器不可用：演示模式开启
                store.updateUser({ notifyEnabled: true })
                self.setData({ notifyOn: true })
                wx.showToast({ title: '已开启报警通知（演示，服务器未连接）', icon: 'none' })
            })
    },

    goMall() {
        wx.switchTab({ url: '/pages/category/category' })
    },

    goInvite() {
        wx.navigateTo({ url: '/pages/invite/invite' })
    },

    refreshCartBadge() {
        const count = store.calcCartCount()
        // 自定义 tabBar：直接更新组件内的购物车角标
        if (typeof this.getTabBar === 'function' && this.getTabBar()) {
            this.getTabBar().setData({ cartCount: count })
            return
        }
        // 原生 tabBar 兜底
        if (count > 0) {
            wx.setTabBarBadge({ index: 2, text: String(count > 99 ? '99+' : count), fail: () => {} })
        } else {
            wx.removeTabBarBadge({ index: 2, fail: () => {} })
        }
    },
})
