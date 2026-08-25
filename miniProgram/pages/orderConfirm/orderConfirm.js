// pages/orderConfirm/orderConfirm.js
const store = require('../../utils/store.js')

Page({
    data: {
        from: '',
        items: [],
        total: '0.00',
        address: null,
        remark: '',
        canSubmit: true,
        rateText: store.RATE_TEXT,
    },

    onLoad(options) {
        this.setData({ from: options.from || '' })
        let items = []
        if (options.from === 'cart') {
            const ids = (options.ids || '').split(',')
            items = store.getCartDetail().filter((i) => ids.indexOf(i.id) > -1)
        } else if (options.from === 'goods') {
            const goods = store.getProductCache(options.goodsId)
            const count = Number(options.count) || 1
            let attrs = []
            if (options.attrs) {
                try {
                    attrs = JSON.parse(decodeURIComponent(options.attrs))
                } catch (e) {
                    attrs = []
                }
            }
            if (goods) {
                items = [
                    {
                        id: goods.id + '_buy',
                        productId: goods.id,
                        name: goods.name,
                        price: goods.price,
                        emoji: goods.emoji,
                        colors: goods.colors,
                        originalPrice: goods.originalPrice,
                        count,
                        attrs,
                        attrsText: store.attrsText(attrs),
                        service: goods.service,
                        sourceTeam: goods.sourceTeam,
                        images: goods.images || [],
                    },
                ]
            }
        }
        const total = items.reduce((s, i) => s + i.price * i.count, 0)
        this.setData({
            items,
            total: store.formatMoney(total),
        })
        // 地址绑定账号保存在服务器，先同步服务器地址再取默认
        store.syncAddressesFromServer().then((list) => {
            const address = list.find((a) => a.isDefault) || list[0] || null
            if (address) this.setData({ address })
        })
    },

    onShow() {
        // 从地址选择页返回时刷新
        const selectedId = wx.getStorageSync('selected_address_id')
        if (selectedId) {
            wx.removeStorageSync('selected_address_id')
            store.syncAddressesFromServer().then((list) => {
                const address = list.find((a) => a.id === selectedId) || list.find((a) => a.isDefault) || list[0] || null
                if (address) this.setData({ address })
            })
            return
        }
        store.syncAddressesFromServer().then((list) => {
            const address = list.find((a) => a.isDefault) || list[0] || null
            if (address && (!this.data.address || this.data.address.id !== address.id)) {
                this.setData({ address })
            }
        })
    },

    chooseAddress() {
        wx.navigateTo({ url: '/pages/address/address?mode=select' })
    },

    onRemarkInput(e) {
        this.setData({ remark: e.detail.value })
    },

    submit() {
        // 提交订单需登录
        if (!store.requireLogin()) return
        if (!this.data.items.length) return
        if (!this.data.address) {
            wx.showToast({ title: '请先填写收货地址', icon: 'none' })
            return
        }
        this.setData({ canSubmit: false })
        // 服务器下单：价格以服务器商品表为准，地址/商品快照持久化
        store
            .createOrderServer({
                items: this.data.items,
                addressId: this.data.address.id,
                remark: this.data.remark,
            })
            .then((order) => {
                // 从购物车购买时移除已下单商品
                if (this.data.from === 'cart') {
                    store.removeCartItems(this.data.items.map((i) => i.id))
                }
                wx.redirectTo({
                    url: '/pages/orderDetail/orderDetail?id=' + order.id,
                })
            })
            .catch((err) => {
                wx.showToast({ title: (err && err.message) || '下单失败', icon: 'none' })
            })
            .finally(() => this.setData({ canSubmit: true }))
    },
})
