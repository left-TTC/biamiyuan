// pages/user/user.js
const store = require('../../utils/store.js')

Page({
    data: {
        user: null,
        isLogin: false,
        balance: '0.00',
        teamCount: 0,
        myTeamName: '',
        rateText: store.RATE_TEXT,
        orderCounts: { pending: 0, paid: 0, shipped: 0, done: 0 },
    },

    onShow() {
        // 同步自定义 tabBar 选中态（我的 = 3）与购物车角标
        if (typeof this.getTabBar === 'function' && this.getTabBar()) {
            this.getTabBar().setData({ selected: 3, cartCount: store.calcCartCount() })
        }
        // 游客可查看个人中心；订单/邀请/提现等操作时要求登录
        const isLogin = store.isLoggedIn()
        const user = isLogin ? store.getUser() : null
        this.setData({
            user,
            isLogin,
            balance: store.formatMoney(user ? user.balance : 0),
            teamCount: 0,
            orderCounts: { pending: 0, paid: 0, shipped: 0, done: 0 },
        })
        // 账号数据一律以服务器为准：本地缓存是账号共享的，不能作为展示来源
        if (isLogin) {
            // 订单角标：来自服务器（切换账号/登出时本地订单缓存已清空）
            store.fetchOrdersServer('all').then((orders) => {
                const count = (s) => (orders || []).filter((o) => o.status === s).length
                this.setData({
                    orderCounts: {
                        pending: count('pending'),
                        paid: count('paid'),
                        shipped: count('shipped'),
                        done: count('done'),
                    },
                })
            })
            // 邀请好友数 = 服务端持久化的真实被邀请人数（与邀请页同一口径，不再读取本地模拟数据）
            store.fetchMyInvitees().then((invitees) => {
                const total = (invitees || []).length
                if (total !== this.data.teamCount) this.setData({ teamCount: total })
            })
            // 团队栏目副标题：显示我所在团队名
            store.getMyTeam().then((t) => this.setData({ myTeamName: t ? t.name : '' }))
        }
    },

    goLogin() {
        wx.navigateTo({ url: '/pages/login/login' })
    },

    goOrderList(e) {
        // 订单需登录
        if (!store.requireLogin()) return
        const status = e.currentTarget.dataset.status || 'all'
        wx.navigateTo({ url: '/pages/orderList/orderList?status=' + status })
    },

    goInvite() {
        // 邀请需登录
        if (!store.requireLogin()) return
        wx.navigateTo({ url: '/pages/invite/invite' })
    },

    goTeam() {
        if (!store.requireLogin()) return
        wx.navigateTo({ url: '/pages/team/team' })
    },

    goCommission() {
        if (!store.requireLogin()) return
        wx.navigateTo({ url: '/pages/commission/commission' })
    },

    goWithdraw() {
        if (!store.requireLogin()) return
        wx.navigateTo({ url: '/pages/withdraw/withdraw' })
    },

    goAddress() {
        if (!store.requireLogin()) return
        wx.navigateTo({ url: '/pages/address/address' })
    },

    goSupport() {
        if (!store.requireLogin()) return
        wx.navigateTo({ url: '/pages/support/support' })
    },

    copyCode() {
        if (!this.data.user) return
        const code = this.data.user.promoterCode
        wx.setClipboardData({
            data: code,
            success: () => {
                wx.showToast({ title: '邀请码已复制', icon: 'success' })
            },
        })
    },

    editProfile() {
        // 编辑头像/昵称需登录
        if (!store.requireLogin()) return
        wx.navigateTo({ url: '/pages/profile/profile' })
    },

    logout() {
        wx.showModal({
            title: '提示',
            content: '确定退出登录吗？',
            success: (res) => {
                if (res.confirm) {
                    store.logout()
                    wx.reLaunch({ url: '/pages/login/login' })
                }
            },
        })
    },
})
