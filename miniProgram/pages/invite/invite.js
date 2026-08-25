// pages/invite/invite.js
// 邀请页：分享邀请 + 绑定邀请码 + 我邀请的好友列表（与团队页职责分离）
const store = require('../../utils/store.js')

Page({
    data: {
        user: null,
        rateText: store.RATE_TEXT,
        minAmount: store.WITHDRAW_MIN,
        balance: '0.00',
        teamCount: 0,
        invitedMembers: [], // 我邀请的好友列表
        inviterInput: '',
        settleDays: 7, // 无理由退货期/佣金到账天数（服务器配置）
    },

    onLoad() {
        // 开启分享菜单：右上角「⋯」可转发给好友 / 分享到朋友圈
        wx.showShareMenu({
            withShareTicket: false,
            menus: ['shareAppMessage', 'shareTimeline'],
        })
    },

    onShow() {
        // 邀请需登录（获取个人邀请码）
        if (!store.requireLogin()) return
        const user = store.getUser()
        this.setData({
            user,
            balance: store.formatMoney(user.balance),
            teamCount: 0,
            invitedMembers: [],
        })
        // 我邀请的好友 = 服务端持久化的真实被邀请人（含消费/佣金统计），不再读取本地模拟数据
        store.fetchMyInvitees().then((invitees) => {
            const members = (invitees || []).map((m) => ({
                id: m.userId,
                nickName: m.nickName || '好友',
                avatarUrl: '👤',
                joinTime: m.joinTime,
                totalSpend: m.totalSpend,
                commission: m.commission,
                pendingCommission: m.pendingCommission,
                simulated: false,
                server: true,
            }))
            this.setData({
                invitedMembers: members,
                teamCount: members.length,
            })
        })
        // 无理由退货期/佣金到账天数（服务器配置）
        store.fetchCommissions().then((res) => {
            if (res && res.settleDays) this.setData({ settleDays: res.settleDays })
        })
    },

    // 统一邀请机制：所有会员均可分享邀请
    formatMoney(n) {
        return store.formatMoney(n)
    },

    onShareAppMessage() {
        return {
            title: '安全监护产品商城，快来和我一起守护家人',
            path: '/pages/index/index?inviter=' + store.myCode(),
        }
    },

    // 分享到朋友圈
    onShareTimeline() {
        return {
            title: '安全监护产品商城，快来和我一起守护家人',
        }
    },

    copyCode() {
        wx.setClipboardData({
            data: this.data.user.promoterCode,
            success: () => wx.showToast({ title: '邀请码已复制', icon: 'success' }),
        })
    },

    onInviterInput(e) {
        this.setData({ inviterInput: e.detail.value })
    },

    // 手动绑定邀请码（被邀请人使用；已登录用户通过邀请链接进入时也会自动绑定）
    bindInvite() {
        const code = (this.data.inviterInput || '').trim()
        store
            .bindInviter(code)
            .then((res) => {
                wx.showToast({ title: res.msg, icon: 'success' })
                this.setData({ inviterInput: '' })
                this.onShow()
            })
            .catch((err) => {
                wx.showToast({ title: (err && err.message) || '绑定失败', icon: 'none' })
            })
    },

    goCommission() {
        wx.navigateTo({ url: '/pages/commission/commission' })
    },

    goWithdraw() {
        wx.navigateTo({ url: '/pages/withdraw/withdraw' })
    },
})
