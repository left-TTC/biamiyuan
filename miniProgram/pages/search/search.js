// pages/search/search.js
const store = require('../../utils/store.js')

Page({
    data: {
        keyword: '',
        searched: false,
        showSuggest: false,
        suggestions: [],
        history: [],
        hotWords: ['智能摄像头', '烟雾报警', '跌倒手环', '定位手表', '可视门铃', '燃气报警'],
        results: [],
        sortedResults: [],
        sortKey: 'default', // default / sales / price
        priceAsc: true,
    },

    onLoad() {
        this.setData({ history: store.getSearchHistory() })
    },

    onShow() {
        this.setData({ history: store.getSearchHistory() })
    },

    // 输入时实时联想
    onInput(e) {
        const keyword = e.detail.value
        this.setData({ keyword })
        const kw = keyword.trim()
        if (kw) {
            const suggestions = store
                .searchProductsCache(kw)
                .slice(0, 8)
                .map((p) => p.name)
            this.setData({ showSuggest: true, suggestions })
        } else {
            this.setData({ showSuggest: false, suggestions: [] })
        }
    },

    onConfirm() {
        this.onSearch()
    },

    onSearch() {
        const kw = this.data.keyword.trim()
        if (!kw) {
            wx.showToast({ title: '请输入搜索关键词', icon: 'none' })
            return
        }
        const results = store.searchProductsCache(kw)
        this.setData({
            results,
            searched: true,
            showSuggest: false,
            sortKey: 'default',
            priceAsc: true,
            history: store.saveSearchHistory(kw),
        })
        this.applySort()
    },

    tapSuggest(e) {
        this.setData({ keyword: e.currentTarget.dataset.word })
        this.onSearch()
    },

    tapHistory(e) {
        this.setData({ keyword: e.currentTarget.dataset.word })
        this.onSearch()
    },

    tapHot(e) {
        this.setData({ keyword: e.currentTarget.dataset.word })
        this.onSearch()
    },

    removeHistory(e) {
        const word = e.currentTarget.dataset.word
        this.setData({ history: store.removeSearchHistory(word) })
    },

    clearHistory() {
        wx.showModal({
            title: '提示',
            content: '确定清空搜索历史吗？',
            success: (res) => {
                if (res.confirm) this.setData({ history: store.clearSearchHistory() })
            },
        })
    },

    clearKeyword() {
        this.setData({
            keyword: '',
            showSuggest: false,
            suggestions: [],
            searched: false,
            results: [],
            sortedResults: [],
        })
    },

    // 综合 / 销量 / 价格排序（价格支持升降切换）
    changeSort(e) {
        const key = e.currentTarget.dataset.key
        if (key === 'price' && this.data.sortKey === 'price') {
            this.setData({ priceAsc: !this.data.priceAsc })
        } else {
            this.setData({ sortKey: key })
        }
        this.applySort()
    },

    applySort() {
        const { results, sortKey, priceAsc } = this.data
        let list = results.slice()
        if (sortKey === 'sales') {
            list.sort((a, b) => b.sales - a.sales)
        } else if (sortKey === 'price') {
            list.sort((a, b) => (priceAsc ? a.price - b.price : b.price - a.price))
        }
        this.setData({ sortedResults: list })
    },

    goGoods(e) {
        wx.navigateTo({ url: '/pages/goods/goods?id=' + e.currentTarget.dataset.id })
    },
})
