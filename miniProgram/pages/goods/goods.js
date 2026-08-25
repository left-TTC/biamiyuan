// pages/goods/goods.js
const store = require('../../utils/store.js')

Page({
    data: {
        goods: null,
        count: 1,
        cartCount: 0,
        rateText: store.RATE_TEXT,
        showPopup: false,
        selectedAttrs: {}, // {属性名: 选中的取值}
        attrsReady: false, // 属性是否已选全
        selectedAttrsText: '',
    },

    onLoad(options) {
        this.productId = options.id
        this.loadGoods()
    },

    loadGoods() {
        const goods = store.getProductCache(this.productId)
        if (goods) {
            this.setData({ goods: this.normalizeGoods(goods) })
            return
        }
        store.initProductCache().then((res) => {
            const g = store.getProductCache(this.productId)
            if (g) {
                this.setData({ goods: this.normalizeGoods(g) })
            } else {
                wx.showToast({
                    title: res.ok ? '商品不存在' : '商品加载失败，请确认本地服务器已启动',
                    icon: 'none',
                })
                setTimeout(() => wx.navigateBack(), 1500)
            }
        })
    },

    // 规范化字段（兼容旧缓存无 attributes/images 字段）
    normalizeGoods(g) {
        const goods = Object.assign({}, g)
        goods.attributes = goods.attributes || []
        goods.images = goods.images || []
        goods.colors = goods.colors && goods.colors.length >= 2 ? goods.colors : ['#3B82F6', '#1E40AF']
        goods.tags = goods.tags || []
        return goods
    },

    onShow() {
        // 游客可浏览商品详情；加购/购买时要求登录
        this.setData({ cartCount: store.calcCartCount() })
    },

    addCount() {
        this.setData({ count: this.data.count + 1 })
    },

    minusCount() {
        if (this.data.count > 1) this.setData({ count: this.data.count - 1 })
    },

    openPopup() {
        const goods = this.data.goods
        const selectedAttrs = {}
        if (goods && goods.attributes) {
            goods.attributes.forEach((a) => {
                // 默认不选，由用户选择；预选第一项更便捷
                selectedAttrs[a.name] = a.values[0]
            })
        }
        const text = store.attrsText(
            Object.keys(selectedAttrs).map((n) => ({ name: n, value: selectedAttrs[n] }))
        )
        this.setData({
            showPopup: true,
            selectedAttrs,
            selectedAttrsText: text,
            attrsReady: store.attrsComplete(goods, Object.keys(selectedAttrs).map((n) => ({ name: n, value: selectedAttrs[n] }))),
        })
    },

    closePopup() {
        this.setData({ showPopup: false })
    },

    noop() {},

    // 选择属性取值（取消已选中的同一取值）
    onAttrSelect(e) {
        const { attr, value } = e.currentTarget.dataset
        const selectedAttrs = Object.assign({}, this.data.selectedAttrs)
        selectedAttrs[attr] = selectedAttrs[attr] === value ? '' : value
        const list = Object.keys(selectedAttrs).map((n) => ({ name: n, value: selectedAttrs[n] }))
        this.setData({
            selectedAttrs,
            selectedAttrsText: store.attrsText(list),
            attrsReady: store.attrsComplete(this.data.goods, list),
        })
    },

    selectedAttrsList() {
        return Object.keys(this.data.selectedAttrs).map((n) => ({ name: n, value: this.data.selectedAttrs[n] }))
    },

    addToCart() {
        // 加入购物车需登录
        if (!store.requireLogin()) return
        if (!this.data.attrsReady) {
            wx.showToast({ title: '请选择完整商品属性', icon: 'none' })
            return
        }
        store.addToCart(this.data.goods.id, this.data.count, this.selectedAttrsList())
        this.setData({ showPopup: false, cartCount: store.calcCartCount() })
        wx.showToast({ title: '已加入购物车', icon: 'success' })
    },

    buyNow() {
        // 立即购买需登录
        if (!store.requireLogin()) return
        if (!this.data.attrsReady) {
            wx.showToast({ title: '请选择完整商品属性', icon: 'none' })
            return
        }
        const { id } = this.data.goods
        const attrs = encodeURIComponent(JSON.stringify(this.selectedAttrsList()))
        this.setData({ showPopup: false })
        wx.navigateTo({
            url:
                '/pages/orderConfirm/orderConfirm?from=goods&goodsId=' +
                id +
                '&count=' +
                this.data.count +
                '&attrs=' +
                attrs,
        })
    },

    goCart() {
        wx.switchTab({ url: '/pages/cart/cart' })
    },

    // 联系客服：服务商品 → 团队指定客服成员；普通商品 → 后台客服
    contactSupport() {
        const g = this.data.goods
        if (!g) return
        if (!store.requireLogin()) return
        wx.navigateTo({
            url:
                '/pages/support/support?productId=' +
                g.id +
                '&productName=' +
                encodeURIComponent(g.name || '') +
                (g.service && g.sourceTeam ? '&sourceTeam=' + encodeURIComponent(g.sourceTeam) : ''),
        })
    },
})
