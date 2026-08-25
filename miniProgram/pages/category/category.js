// pages/category/category.js
const store = require('../../utils/store.js')

Page({
    data: {
        categories: [], // 一级类目
        subCats: [], // 当前一级类目下的二级类目
        current: null, // 当前一级类目
        currentSub: null, // 当前二级类目
        products: [],
        activeIndex: 0, // 一级类目选中下标
        subActiveIndex: 0, // 二级类目选中下标
        cacheError: false,
        usingFallback: false,
    },

    onLoad() {
        this.loadData()
    },

    onShow() {
        // 同步自定义 tabBar 选中态（商城 = 1）与购物车角标
        if (typeof this.getTabBar === 'function' && this.getTabBar()) {
            this.getTabBar().setData({ selected: 1, cartCount: store.calcCartCount() })
        }
        // 游客可浏览商城
        const pending = wx.getStorageSync('pending_category')
        if (pending) {
            wx.removeStorageSync('pending_category')
            this.jumpToCategory(pending)
        }
    },

    // 商品数据：每次启动小程序后首次进入本页时从服务器拉取并刷新本地缓存；
    // 之后进入直接使用本地缓存，不再请求服务器（retry 可强制重新拉取）
    loadData(force) {
        if (this.loadingCache) return
        this.loadingCache = true
        store.fetchProductsOnce(force).then((res) => {
            this.loadingCache = false
            const level1 = store.getLevel1Categories()
            if (!level1.length) {
                // 本地也没有任何缓存数据：提示加载失败，可点击重试强制重新拉取
                this.setData({ cacheError: true })
                return
            }
            this.renderCategories(level1)
        })
    },

    renderCategories(level1) {
        this.setData({
            cacheError: false,
            categories: level1,
        })
        // 若存在待跳转的类目（分享直达等），优先定位到它
        const pending = wx.getStorageSync('pending_category')
        if (pending) {
            wx.removeStorageSync('pending_category')
            this.jumpToCategory(pending)
            return
        }
        this.switchCategory(level1[0], 0)
    },

    // 按类目 ID 定位（支持一级或二级类目 ID）
    jumpToCategory(id) {
        const level1 = store.getLevel1Categories()
        if (!level1.length) return
        const cat = store.getCategoriesCache().find((c) => c.id === id)
        if (!cat) {
            this.switchCategory(level1[0], 0)
            return
        }
        if (cat.parentId) {
            // 二级类目：先切到其一级类目，再选中该二级类目
            const pIdx = level1.findIndex((c) => c.id === cat.parentId)
            this.switchCategory(level1[pIdx >= 0 ? pIdx : 0], pIdx >= 0 ? pIdx : 0)
            const subs = store.getLevel2Categories(cat.parentId)
            const sIdx = subs.findIndex((c) => c.id === id)
            if (sIdx >= 0) this.switchSub(subs[sIdx], sIdx)
        } else {
            const idx = level1.findIndex((c) => c.id === id)
            this.switchCategory(level1[idx >= 0 ? idx : 0], idx >= 0 ? idx : 0)
        }
    },

    // 切换一级类目：展示其二级类目，默认选中第一个
    switchCategory(category, index) {
        if (!category) return
        const subCats = store.getLevel2Categories(category.id)
        const defaultSub = subCats[0] || null
        this.setData({
            current: category,
            activeIndex: index,
            subCats,
            subActiveIndex: 0,
            currentSub: defaultSub,
            products: defaultSub ? store.getProductsByCategoryCache(defaultSub.id) : [],
        })
    },

    // 切换二级类目：展示其下商品
    switchSub(sub, index) {
        if (!sub) return
        this.setData({
            currentSub: sub,
            subActiveIndex: index,
            products: store.getProductsByCategoryCache(sub.id),
        })
    },

    selectCategory(e) {
        const { id, index } = e.currentTarget.dataset
        const category = this.data.categories[index]
        if (category && category.id !== (this.data.current || {}).id) {
            this.switchCategory(category, index)
        }
    },

    selectSubCategory(e) {
        const { id, index } = e.currentTarget.dataset
        const sub = this.data.subCats[index]
        if (sub && sub.id !== (this.data.currentSub || {}).id) {
            this.switchSub(sub, index)
        }
    },

    retry() {
        this.setData({ cacheError: false })
        this.loadData(true)
    },

    goSearch() {
        wx.navigateTo({ url: '/pages/search/search' })
    },

    goGoods(e) {
        wx.navigateTo({ url: '/pages/goods/goods?id=' + e.currentTarget.dataset.id })
    },
})
