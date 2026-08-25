// pages/withdraw/withdraw.js
const store = require('../../utils/store.js')
const util = require('../../utils/util.js')

Page({
    data: {
        balance: '0.00',
        amount: '',
        method: 'wechat',
        account: '',
        minAmount: store.WITHDRAW_MIN,
        methods: [
            { key: 'wechat', label: '微信零钱', icon: '💚' },
            { key: 'alipay', label: '支付宝', icon: '💙' },
            { key: 'bank', label: '银行卡', icon: '🏦' },
        ],
        records: [],
    },

    onShow() {
        // 余额与提现记录与账号绑定：需登录后查看
        if (!store.requireLogin()) return
        this.setData({
            balance: store.formatMoney(store.getUser().balance),
        })
        // 提现记录来自服务器（服务器写入企业银行卡号，状态由后台审核）
        store.fetchWithdrawalsServer().then((list) => {
            this.setData({
                records: (list || []).map((r) =>
                    Object.assign({}, r, {
                        methodText: store.methodName(r.method),
                        amountText: store.formatMoney(r.amount),
                        statusText: { processing: '处理中', done: '已到账', failed: '已驳回' }[r.status] || r.status,
                        timeText: util.formatTime(new Date(r.applyTime)),
                    })
                ),
            })
        })
    },

    onAmount(e) {
        this.setData({ amount: e.detail.value })
    },

    onAccount(e) {
        this.setData({ account: e.detail.value })
    },

    selectMethod(e) {
        this.setData({ method: e.currentTarget.dataset.key })
    },

    allIn() {
        this.setData({ amount: String(store.getUser().balance) })
    },

    submit() {
        const amount = Number(this.data.amount)
        if (!amount || amount < this.data.minAmount) {
            return wx.showToast({ title: '最低提现金额为 ¥' + this.data.minAmount, icon: 'none' })
        }
        if (amount > store.getUser().balance) {
            return wx.showToast({ title: '可提现余额不足', icon: 'none' })
        }
        // 提现由服务器处理：事务内扣减余额，绑定 .env 企业银行卡号，后台审核打款
        store
            .applyWithdrawServer({
                amount,
                method: this.data.method,
                account: this.data.account.trim(),
            })
            .then(() => {
                wx.showToast({ title: '提现申请已提交', icon: 'success' })
                this.setData({ amount: '', account: '' })
                this.onShow()
            })
            .catch((err) => {
                wx.showToast({ title: (err && err.message) || '提现失败', icon: 'none' })
            })
    },
})
