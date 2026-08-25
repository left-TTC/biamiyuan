// pages/cart/cart.js
const store = require('../../utils/store.js')

Page({
    data: {
        cart: [],
        totalPrice: '0.00',
        allChecked: false,
        totalCount: 0,
    },

    onShow() {
        // 同步自定义 tabBar 选中态（购物车 = 2）与购物车角标
        if (typeof this.getTabBar === 'function' && this.getTabBar()) {
            this.getTabBar().setData({ selected: 2, cartCount: store.calcCartCount() })
        }
        // 游客可浏览购物车；结算时要求登录
        this.refresh()
        // 已登录：从服务器拉取购物车历史并恢复（覆盖"重新登录后本地购物车缺失"的场景）
        if (store.isLoggedIn()) {
            store.fetchServerCart().then(() => this.refresh())
        }
    },

    refresh() {
        const cart = store.getCartDetail()
        const totalPrice = store.calcCartTotal()
        const allChecked = cart.length > 0 && cart.every((item) => item.checked)
        this.setData({
            cart,
            totalPrice: store.formatMoney(totalPrice),
            allChecked,
            totalCount: cart.filter((i) => i.checked && !i.invalid).length,
        })
    },

    toggleItem(e) {
        store.toggleCartChecked(e.currentTarget.dataset.id)
        this.refresh()
    },

    toggleAll() {
        store.setCartCheckedAll(!this.data.allChecked)
        this.refresh()
    },

    changeCount(e) {
        const { id, delta } = e.currentTarget.dataset
        const item = this.data.cart.find((c) => c.id === id)
        if (!item) return
        const count = (item.count || 1) + Number(delta)
        if (count < 1) return
        store.updateCartCount(id, count)
        this.refresh()
    },

    removeItem(e) {
        const id = e.currentTarget.dataset.id
        wx.showModal({
            title: '提示',
            content: '确定将该商品移出购物车吗？',
            success: (res) => {
                if (res.confirm) {
                    store.removeCartItems([id])
                    this.refresh()
                }
            },
        })
    },

    checkout() {
        // 结算需登录
        if (!store.requireLogin()) return
        const checked = this.data.cart.filter((i) => i.checked && !i.invalid)
        if (!checked.length) {
            wx.showToast({ title: '请先选择商品', icon: 'none' })
            return
        }
        // 属性未选全（服务器恢复的购物车项可能缺规格）需重新选择
        if (checked.some((i) => i.missingSpec)) {
            wx.showModal({
                title: '商品规格待选择',
                content: '部分商品需重新选择规格（尺码/版本等），请删除后重新加入购物车',
                showCancel: false,
            })
            return
        }
        const ids = checked.map((c) => c.id).join(',')
        wx.navigateTo({ url: '/pages/orderConfirm/orderConfirm?from=cart&ids=' + ids })
    },

    goShopping() {
        wx.switchTab({ url: '/pages/category/category' })
    },
})
