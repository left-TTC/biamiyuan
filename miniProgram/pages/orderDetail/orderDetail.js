// pages/orderDetail/orderDetail.js
const store = require('../../utils/store.js')
const util = require('../../utils/util.js')

const STATUS_MAP = {
    pending: '待付款',
    paid: '待发货',
    shipped: '待收货',
    done: '已完成',
    canceled: '已取消',
    refunded: '已退款',
}

const STATUS_EMOJI = {
    pending: '💰',
    paid: '📦',
    shipped: '🚚',
    done: '✅',
    canceled: '🗑️',
    refunded: '↩️',
}

const STATUS_DESC = {
    pending: '请尽快完成支付，超时订单将自动取消',
    paid: '商家正在加急备货，请耐心等待',
    shipped: '商品已发出，请注意查收',
    done: '交易完成，感谢您的信任',
    canceled: '订单已取消',
    refunded: '订单已退款（关联的待结算佣金已取消）',
}

Page({
    data: {
        order: null,
        statusText: '',
        statusEmoji: '',
        statusDesc: '',
        timeText: '',
        payTimeText: '',
        shipText: '',
        canPay: false,
        canCancel: false,
        canConfirm: false,
        canRefund: false,
    },

    onLoad(options) {
        this.load(options.id)
    },

    onShow() {
        if (this.data.order) this.load(this.data.order.id)
    },

    load(id) {
        store.fetchOrderServer(id).then((order) => {
            if (!order) {
                wx.showToast({ title: '订单不存在', icon: 'none' })
                setTimeout(() => wx.navigateBack(), 800)
                return
            }
            const shipText =
                order.status === 'shipped' && order.shipCompany && order.shipNo
                    ? order.shipCompany + ' · ' + order.shipNo
                    : ''
            this.setData({
                order,
                statusText: STATUS_MAP[order.status],
                statusEmoji: STATUS_EMOJI[order.status],
                statusDesc: STATUS_DESC[order.status],
                timeText: util.formatTime(new Date(order.createTime)),
                payTimeText: order.payTime ? util.formatTime(new Date(order.payTime)) : '',
                shipText,
                canPay: order.status === 'pending',
                canCancel: order.status === 'pending',
                canConfirm: order.status === 'shipped',
                canRefund: order.status === 'paid', // 待发货订单可无理由退款（退货期内）
            })
        })
    },

    pay() {
        const order = this.data.order
        if (!order) return
        wx.showLoading({ title: '正在拉起支付...' })
        store
            .payOrderServer(order.id, 'wechat')
            .then((res) => {
                wx.hideLoading()
                // 真实微信支付：服务器返回 wx.requestPayment 参数
                if (res && res.mode === 'real') {
                    wx.requestPayment({
                        timeStamp: res.timeStamp,
                        nonceStr: res.nonceStr,
                        package: res.package,
                        signType: res.signType,
                        paySign: res.paySign,
                        success: () => {
                            wx.showToast({ title: '支付成功', icon: 'success' })
                            // 微信回调确认订单已支付，稍后刷新订单状态
                            setTimeout(() => this.load(order.id), 1500)
                        },
                        fail: (err) => {
                            const msg = (err && err.errMsg) || ''
                            wx.showToast({
                                title: msg.indexOf('cancel') >= 0 ? '已取消支付' : '支付失败',
                                icon: 'none',
                            })
                            this.load(order.id)
                        },
                    })
                    return
                }
                // 模拟模式：服务器直接确认支付成功
                wx.showToast({ title: '支付成功', icon: 'success' })
                this.load(order.id)
            })
            .catch((err) => {
                wx.hideLoading()
                wx.showToast({ title: (err && err.message) || '支付失败', icon: 'none' })
            })
    },

    cancel() {
        wx.showModal({
            title: '提示',
            content: '确定取消该订单吗？',
            success: (res) => {
                if (res.confirm) {
                    store
                        .cancelOrderServer(this.data.order.id)
                        .then(() => this.load(this.data.order.id))
                        .catch((err) => {
                            wx.showToast({ title: (err && err.message) || '取消失败', icon: 'none' })
                        })
                }
            },
        })
    },

    confirmReceive() {
        wx.showModal({
            title: '提示',
            content: '确认已收到商品？',
            success: (res) => {
                if (res.confirm) {
                    store
                        .confirmOrderServer(this.data.order.id)
                        .then(() => {
                            wx.showToast({ title: '确认成功', icon: 'success' })
                            this.load(this.data.order.id)
                        })
                        .catch((err) => {
                            wx.showToast({ title: (err && err.message) || '操作失败', icon: 'none' })
                        })
                }
            },
        })
    },

    // 无理由退款（支付后无理由退货期内可退；退款后订单关联的待结算佣金自动取消）
    refund() {
        const order = this.data.order
        if (!order) return
        wx.showModal({
            title: '申请退款',
            content: '确认对该订单申请无理由退款吗？退款后订单关联的待结算佣金将自动取消。',
            success: (res) => {
                if (!res.confirm) return
                store
                    .refundOrder(order.id)
                    .then(() => {
                        wx.showToast({ title: '退款成功', icon: 'success' })
                        this.load(order.id)
                    })
                    .catch((err) => {
                        wx.showToast({ title: (err && err.message) || '退款失败', icon: 'none' })
                    })
            },
        })
    },

    // 联系客服：普通商品 → 后台客服；团队服务 → 团队指定客服成员
    contactSupport() {
        const order = this.data.order
        if (!order) return
        const first = (order.items || [])[0] || {}
        const source = first.service && first.sourceTeam ? '&sourceTeam=' + encodeURIComponent(first.sourceTeam) : ''
        wx.navigateTo({
            url:
                '/pages/support/support?productId=' +
                (first.productId || '') +
                '&productName=' +
                encodeURIComponent(first.name || '') +
                source,
        })
    },

    goHome() {
        wx.switchTab({ url: '/pages/index/index' })
    },
})
