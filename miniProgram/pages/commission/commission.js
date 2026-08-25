// pages/commission/commission.js
const store = require('../../utils/store.js')
const util = require('../../utils/util.js')

const STATUS_TEXT = {
    pending: '待结算',
    settled: '已到账',
    cancelled: '已取消',
}

Page({
    data: {
        list: [],
        income: '0.00',
        pendingAmount: '0.00',
        balance: '0.00',
        settleDays: 7,
    },

    onShow() {
        // 佣金数据与账号绑定：需登录后查看（服务端持久化）
        if (!store.requireLogin()) return
        const user = store.getUser()
        this.setData({ balance: store.formatMoney(user.balance) })
        store.fetchCommissions().then((res) => {
            const days = (res && res.settleDays) || 7
            const list = ((res && res.list) || []).map((c) => ({
                id: c.id,
                orderId: c.orderId,
                amount: c.amount,
                status: c.status,
                timeText: util.formatTime(new Date(c.paidAt || c.createdAt)),
                statusText: STATUS_TEXT[c.status] || c.status,
                title:
                    c.status === 'pending'
                        ? '订单佣金（待结算，' + days + ' 天后到账）'
                        : c.status === 'settled'
                          ? '订单佣金（已到账）'
                          : '订单佣金（退款已取消）',
                amountText: (c.status === 'cancelled' ? '' : '+') + store.formatMoney(c.amount),
            }))
            const settled = ((res && res.list) || []).filter((c) => c.status === 'settled')
            const pending = ((res && res.list) || []).filter((c) => c.status === 'pending')
            this.setData({
                list,
                income: store.formatMoney(settled.reduce((s, c) => s + c.amount, 0)),
                pendingAmount: store.formatMoney(pending.reduce((s, c) => s + c.amount, 0)),
                settleDays: days,
            })
        })
    },

    goWithdraw() {
        wx.navigateTo({ url: '/pages/withdraw/withdraw' })
    },
})
