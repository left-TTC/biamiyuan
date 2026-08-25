// custom-tab-bar/index.js
// 自定义 tabBar：图标大小可通过 index.wxss 中的 .tab-bar-icon 完全控制
const store = require('../utils/store.js')

Component({
    data: {
        selected: 0,
        cartCount: 0, // 购物车角标（自定义 tabBar 不支持 wx.setTabBarBadge）
        list: [
            { pagePath: '/pages/index/index', text: '首页', iconPath: '/assets/tabbar/index.png', selectedIconPath: '/assets/tabbar/index-active.png' },
            { pagePath: '/pages/category/category', text: '商城', iconPath: '/assets/tabbar/mall.png', selectedIconPath: '/assets/tabbar/mall-active.png' },
            { pagePath: '/pages/cart/cart', text: '购物车', iconPath: '/assets/tabbar/cart.png', selectedIconPath: '/assets/tabbar/cartactive.png' },
            { pagePath: '/pages/user/user', text: '我的', iconPath: '/assets/tabbar/my.png', selectedIconPath: '/assets/tabbar/myactive.png' },
        ],
    },

    // 组件所在页面每次显示时刷新购物车角标
    pageLifetimes: {
        show() {
            this.setData({ cartCount: store.calcCartCount() })
        },
    },

    methods: {
        switchTab(e) {
            const path = e.currentTarget.dataset.path
            wx.switchTab({ url: path })
        },
    },
})
