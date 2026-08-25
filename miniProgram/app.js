// app.js
const store = require('./utils/store.js')

App({
    onLaunch(options) {
        // 不自动创建账号：未登录/未注册时为 null
        this.globalData.user = store.getUser()

        // 商品数据不在启动时请求：由商城页每次启动后首次进入时拉取并缓存

        // 已登录用户：同步服务器（获取 token + 绑定微信 openid，用于订阅消息推送）
        if (store.isLoggedIn()) {
            store.syncServerLogin(store.getUser().phone)
        }

        // 处理分享进入时的邀请码（冷启动）：暂存，待注册/登录时自动绑定
        const query = (options && options.query) || {}
        const inviter = query.inviter
        if (inviter) {
            store.saveInviter(inviter)
            // 已登录用户通过邀请链接进入：直接尝试绑定邀请关系
            if (store.isLoggedIn()) {
                store.bindInviter(inviter).catch(() => {})
            }
        }
    },
    globalData: {
        user: null,
    },
})

