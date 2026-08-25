// pages/orderList/orderList.js
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

Page({
    data: {
        tabs: [
            { key: 'all', label: '全部' },
            { key: 'pending', label: '待付款' },
            { key: 'paid', label: '待发货' },
            { key: 'shipped', label: '待收货' },
            { key: 'done', label: '已完成' },
        ],
        current: 'all',
        orders: [],
    },

    onLoad(options) {
        const status = options.status || 'all'
        this.setData({ current: status })
    },

    onShow() {
        this.refresh()
    },

    refresh() {
        store.fetchOrdersServer(this.data.current).then((list) => {
            const orders = (list || []).map((o) => {
                return Object.assign({}, o, {
                    statusText: STATUS_MAP[o.status] || o.status,
                    timeText: util.formatTime(new Date(o.createTime)),
                })
            })
            this.setData({ orders })
        })
    },

    switchTab(e) {
        this.setData({ current: e.currentTarget.dataset.key })
        this.refresh()
    },

    goDetail(e) {
        wx.navigateTo({ url: '/pages/orderDetail/orderDetail?id=' + e.currentTarget.dataset.id })
    },

    goPay(e) {
        wx.navigateTo({ url: '/pages/orderDetail/orderDetail?id=' + e.currentTarget.dataset.id })
    },
})
