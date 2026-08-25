// utils/store.js
// 安全监护产品商城 · 数据层（服务器为唯一数据源）
//
// 架构约定：
//   1. 所有会变化的业务数据（订单/佣金/提现/地址/设备/设备消息/购物车历史）一律以服务器为准：
//      页面通过 fetchXxxServer() 拉取服务器数据，本地只保留「当前账号」的展示缓存与离线兜底。
//      本地不再有任何"伪造数据"的写入路径（如本地下单/本地提现/本地造佣金）。
//   2. 账号隔离：与账号绑定的本地数据（含用户资料 mall_user_<phone>、购物车/订单/佣金/提现/
//      地址/设备/设备消息/搜索历史）一律以「手机号」作为存储 key 后缀（acctKey/userKey），
//      同一台设备切换账号互不串数据；KEY_ACTIVE_PHONE 为"当前活动账号"指针（会话级，单一 key）。
//      退出登录/切换账号时清理当前（旧）账号缓存，但保留各账号的用户资料快照便于恢复。
const api = require('./api.js')

// ==================== 业务常量 ====================
const COMMISSION_RATE = 0.1 // 邀请订单佣金比例 10%
const WITHDRAW_MIN = 1 // 最低提现金额（元）
const WITHDRAW_FEE = 0 // 提现手续费（元）
const RATE_TEXT = Math.round(COMMISSION_RATE * 100) + '%'
const SMS_CODE = '12345' // 演示验证码（接入真实短信服务后移除）
const SETTLE_DAYS = 7 // 无理由退货期/佣金到账等待天数（默认 7，服务器返回时覆盖）

// ==================== 存储 Key ====================
const KEY_USER = 'mall_user' // 用户资料基础 key：实际按手机号存储为 mall_user_<phone>
const KEY_ACTIVE_PHONE = 'mall_active_phone' // 当前活动账号手机号（会话指针，单一 key，定位当前账号）
const KEY_CART = 'mall_cart'
const KEY_ORDERS = 'mall_orders'
const KEY_COMMISSION = 'mall_commission'
const KEY_WITHDRAWALS = 'mall_withdrawals'
const KEY_ADDRESS = 'mall_address'
const KEY_FIRST_ENTER = 'mall_first_enter'
const KEY_PRODUCTS_CACHE = 'mall_products_cache'
const KEY_CATEGORIES_CACHE = 'mall_categories_cache'
const KEY_SEARCH_HISTORY = 'mall_search_history'

// ==================== 基础读写 ====================
function get(key, def) {
    const val = wx.getStorageSync(key)
    return val === '' || val === null || val === undefined ? def : val
}
function set(key, value) {
    wx.setStorageSync(key, value)
}

// ==================== 账号隔离 ====================
// 与账号绑定的本地数据（购物车/订单/佣金/提现/地址/设备/设备消息）一律按「当前登录手机号」
// 作为存储 key 后缀：同一设备上多个账号各用各的缓存，互不串数据。
// 服务器是唯一数据源，本地缓存仅用于离线兜底与当前账号展示。
function acctKey(base) {
    const user = getUsers()
    const phone = String((user && user.phone) || '').trim()
    return phone ? base + '_' + phone : base + '_guest'
}

// 用户资料存储键：按手机号隔离（mall_user_<phone>），未登录为 mall_user_guest
function userKey(phone) {
    const p = String(phone || '').trim()
    return KEY_USER + '_' + (p || 'guest')
}

// ==================== 工具 ====================
function genId() {
    return 'id_' + Date.now().toString(36) + Math.random().toString(36).slice(2, 8)
}
function genOrderNo() {
    const d = new Date()
    const p = (n) => (n < 10 ? '0' + n : '' + n)
    return (
        d.getFullYear() + p(d.getMonth() + 1) + p(d.getDate()) +
        p(d.getHours()) + p(d.getMinutes()) + p(d.getSeconds()) +
        Math.floor(Math.random() * 1000)
    )
}
function genPromoterCode() {
    const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789'
    let code = ''
    for (let i = 0; i < 6; i++) code += chars[Math.floor(Math.random() * chars.length)]
    return code
}
function formatMoney(n) {
    return Number(n || 0).toFixed(2)
}


// 数据源标记：server 服务器 / fallback 内置兜底
var KEY_DATA_SOURCE = 'mall_data_source'

// get img url from server
function normalizeImages(product) {
    if (!product) return product
    const images = Array.isArray(product.images) ? product.images : []
    return Object.assign({}, product, {
        images: images.map((u) => (u.indexOf('http') === 0 ? u : api.BASE_URL + u)),
    })
}

// 拉取商品与分类并写入本地缓存（成功返回 ok:true）
function initProductCache() {
    return Promise.all([api.request('/api/v1/categories'), api.request('/api/v1/products')])
        .then(([categories, products]) => {
            const prods = (products || []).map(normalizeImages)
            set(KEY_DATA_SOURCE, 'server')
            set(KEY_CATEGORIES_CACHE, categories || [])
            set(KEY_PRODUCTS_CACHE, prods)
            return { ok: true, categories: categories || [], products: prods }
        })
        .catch((err) => {
            console.warn('商品数据拉取失败（使用本地缓存）:', err && err.message)
            return { ok: false, categories: getCategoriesCache(), products: getProductsCache() }
        })
}

// 会话内"商城页首次进入"标记：小程序冷启动时 JS 环境重建，此标记自动归零
let productCacheFetchedThisSession = false

// 每次启动小程序后第一次调用：从服务器拉取并刷新本地缓存；
// 之后（force 为 true 除外）直接使用本地缓存，不再请求服务器
function fetchProductsOnce(force) {
    if (!force && productCacheFetchedThisSession) {
        return Promise.resolve({
            ok: true,
            cached: true,
            categories: getCategoriesCache(),
            products: getProductsCache(),
        })
    }
    productCacheFetchedThisSession = true
    return initProductCache()
}


function getCategoriesCache() {
    return get(KEY_CATEGORIES_CACHE, [])
}

// 一级类目（parentId 为空）
function getLevel1Categories() {
    return getCategoriesCache().filter((c) => !c.parentId)
}

// 二级类目（属于指定一级类目）
function getLevel2Categories(parentId) {
    return getCategoriesCache().filter((c) => c.parentId === parentId)
}

function getProductsCache() {
    return get(KEY_PRODUCTS_CACHE, [])
}

function getProductCache(id) {
    return getProductsCache().find((p) => p.id === id) || null
}

function getProductsByCategoryCache(categoryId) {
    return getProductsCache().filter((p) => p.category === categoryId)
}

function searchProductsCache(keyword) {
    const kw = String(keyword || '').trim().toLowerCase()
    if (!kw) return getProductsCache()
    return getProductsCache().filter(
        (p) =>
            String(p.name).toLowerCase().indexOf(kw) > -1 ||
            String(p.desc).toLowerCase().indexOf(kw) > -1
    )
}

// ==================== 搜索历史（按账号隔离） ====================
function getSearchHistory() {
    return get(acctKey(KEY_SEARCH_HISTORY), [])
}

function saveSearchHistory(keyword) {
    const kw = String(keyword || '').trim()
    if (!kw) return getSearchHistory()
    let history = getSearchHistory().filter((h) => h !== kw)
    history.unshift(kw)
    history = history.slice(0, 10)
    set(acctKey(KEY_SEARCH_HISTORY), history)
    return history
}

function removeSearchHistory(keyword) {
    const history = getSearchHistory().filter((h) => h !== keyword)
    set(acctKey(KEY_SEARCH_HISTORY), history)
    return history
}

function clearSearchHistory() {
    set(acctKey(KEY_SEARCH_HISTORY), [])
    return []
}

// ==================== 服务器登录同步 ====================
const KEY_SERVER_TOKEN = 'mall_server_token'
const KEY_PENDING_INVITER = 'mall_pending_inviter'

function getServerToken() {
    return get(KEY_SERVER_TOKEN, '')
}

function setServerToken(token) {
    set(KEY_SERVER_TOKEN, token)
}

// 已登录用户启动时同步服务器：重新获取 token（用于订阅消息推送），并恢复购物车历史
function syncServerLogin(phone) {
    return api
        .request('/api/v1/auth/code', { method: 'POST', data: { phone }, noRetry: true })
        .then(() => api.request('/api/v1/auth/login', { method: 'POST', data: { phone, code: SMS_CODE }, noRetry: true }))
        .then((res) => {
            if (res && res.token) setServerToken(res.token)
            return bindWxOpenid()
        })
        .then(() => fetchServerCart()) // 冷启动/重新进入：从服务器拉取购物车历史并合并到本地
        .catch((err) => {
            console.warn('服务器登录同步失败（本地演示可继续使用）:', err && err.message)
            return null
        })
}

// 分享冷启动进入时暂存邀请码，等用户注册时自动绑定邀请关系
function saveInviter(code) {
    if (!code) return
    set(KEY_PENDING_INVITER, code)
}

// 取出并清除暂存的邀请码（注册成功后一次性使用）
function takePendingInviter() {
    const code = get(KEY_PENDING_INVITER, '')
    if (code) wx.removeStorageSync(KEY_PENDING_INVITER)
    return code || ''
}

// 用 wx.login 的 code 绑定微信 openid（订阅消息推送目标）
function bindWxOpenid() {
    return new Promise((resolve) => {
        wx.login({
            success: (res) => {
                api
                    .request('/api/v1/auth/wx-login', {
                        method: 'POST',
                        data: { code: res.code },
                        token: getServerToken(),
                    })
                    .then((r) => resolve(r))
                    .catch(() => resolve(null))
            },
            fail: () => resolve(null),
        })
    })
}

// ==================== 用户 ====================
const DEFAULT_AVATAR = 'https://mmbiz.qpic.cn/mmbiz/icTdbqWNOwNRna42FI242Lcia07jQodd2FJGIYQfG0LAJGFxM4FbnQP6yfMxBgJ0F3YRqJCJ1aPAK2dQagdusBZg/0'

// 获取当前活动账号的用户资料（按手机号隔离存储；未登录返回 null）
function getUsers() {
    const phone = String(wx.getStorageSync(KEY_ACTIVE_PHONE) || '').trim()
    if (!phone) return null
    return get(userKey(phone), null)
}

// 保存用户资料并更新活动账号指针（按 userKey 隔离，多账号互不混淆）
function saveUsers(user) {
    if (!user) return null
    const p = String(user.phone || '').trim()
    set(userKey(p), user)
    set(KEY_ACTIVE_PHONE, p)
    return user
}

function initUser() {
    let user = getUsers()
    if (!user) {
        user = {
            id: genId(),
            nickName: '守护会员',
            avatarUrl: DEFAULT_AVATAR,
            phone: '',
            role: 'member', // 统一会员
            promoterCode: genPromoterCode(),
            balance: 0, // 可提现余额（元）
            totalCommission: 0, // 累计获得佣金
            invitedBy: '', // 邀请人推广码：仅在服务器绑定成功后写入（bindInviter），此处不预填
            invitedByName: '',
            createTime: Date.now(),
        }
        set(KEY_FIRST_ENTER, true)
    }
    return user
}

// 获取当前登录用户；未登录（未注册）返回 null，不再自动创建游客账号
function getUser() {
    return getUsers()
}

function updateUser(patch) {
    const user = getUsers()
    if (!user) return null
    const next = Object.assign({}, user, patch)
    return saveUsers(next)
}

function setUserProfile(patch) {
    return updateUser(patch)
}

// 更新用户资料（昵称 / 头像）：本地立即生效，并同步服务器（服务器不可用时本地保留）
// patch: { nickName?, avatarUrl? }
function updateUserProfile(patch) {
    const p = {}
    if (patch.nickName !== undefined && patch.nickName !== null) {
        const nick = String(patch.nickName).trim().slice(0, 20)
        if (nick) p.nickName = nick
    }
    if (patch.avatarUrl !== undefined && patch.avatarUrl !== null) {
        const avatar = String(patch.avatarUrl).trim()
        if (avatar) p.avatarUrl = avatar
    }
    if (!p.nickName && !p.avatarUrl) return getUsers()

    updateUser(p)
    // 同步服务器（演示环境：服务器用户存内存，重启清空；失败不影响本地）
    const token = getServerToken()
    if (token) {
        api
            .request('/api/v1/user/profile', { method: 'PUT', data: p, token, noRetry: true })
            .catch((err) => {
                console.warn('用户资料同步服务器失败（本地已保存）:', err && err.message)
            })
    }
    return getUsers()
}

function isFirstEnter() {
    const first = get(KEY_FIRST_ENTER, false)
    if (first) set(KEY_FIRST_ENTER, false)
    return !!first
}

function myCode() {
    const user = getUsers()
    return user ? user.promoterCode : ''
}

// ==================== 登录 / 注册（以服务器为准）====================
function isLoggedIn() {
    const user = getUsers()
    return !!(user && user.phone)
}

// 需要登录的操作统一入口：未登录则跳转登录页，返回当前是否已登录
function requireLogin() {
    if (isLoggedIn()) return true
    wx.navigateTo({ url: '/pages/login/login' })
    return false
}

// 服务器认证成功后写入本地账号（仅在此处创建本地用户，杜绝游客自动建号）
function applyServerUser(serverUser, phone) {
    let user = getUsers()
    const prevId = user ? user.id : null
    // 记录旧账号手机号：账号数据缓存按手机号隔离（acctKey），
    // 首次注册/切换账号时清理旧账号（或游客）的残留缓存。
    const prevPhone = String((user && user.phone) || '').trim()
    const prevKey = (base) => (prevPhone ? base + '_' + prevPhone : base + '_guest')
    if (!user) {
        // 首次注册/登录：清理历史游客残留数据（设备与账号绑定）
        wx.removeStorageSync(prevKey(KEY_DEVICES))
        wx.removeStorageSync(prevKey(KEY_DEVICE_MSGS))
        user = initUser()
    }
    const p = String(phone || '').trim()
    const next = Object.assign({}, user, {
        id: serverUser.id || user.id,
        phone: p || user.phone,
        nickName: serverUser.nickName || user.nickName,
        avatarUrl: serverUser.avatarUrl || user.avatarUrl,
        role: serverUser.role || 'member',
        promoterCode: serverUser.promoterCode || user.promoterCode,
        balance: Number(serverUser.balance) || 0,
        totalCommission: Number(serverUser.totalCommission) || 0,
        invitedBy: serverUser.inviterCode || user.invitedBy || '', // 优先用服务器返回的邀请人推广码恢复展示
        invitedByName: serverUser.invitedByName || user.invitedByName || '',
        loginTime: Date.now(),
    })
    saveUsers(next) // 写入该账号的用户资料（mall_user_<phone>）并切换活动账号指针
    // 切换账号（登录了不同账号）：本地缓存按手机号隔离，切换时仍清理旧账号缓存，
    // 服务器是唯一数据源，登录后重新拉取，避免把上一账号的数据展示/合并给当前账号。
    if (prevId && serverUser.id && prevId !== serverUser.id) {
        wx.removeStorageSync(prevKey(KEY_ORDERS))
        wx.removeStorageSync(prevKey(KEY_COMMISSION))
        wx.removeStorageSync(prevKey(KEY_WITHDRAWALS))
        wx.removeStorageSync(prevKey(KEY_ADDRESS))
        wx.removeStorageSync(prevKey(KEY_CART))
        serverInviteeCount = 0
    }
    return next
}

// 服务器登录：账号须先在服务器注册，未注册时服务器会返回错误
function loginWithPhone(phone, code) {
    return api
        .request('/api/v1/auth/login', { method: 'POST', data: { phone, code }, noRetry: true })
        .then((res) => {
            setServerToken(res.token)
            applyServerUser(res.user || {}, phone)
            // 受邀人通过邀请链接打开后登录：自动绑定暂存的邀请人（老用户场景）
            const inviter = takePendingInviter()
            if (inviter) {
                bindInviter(inviter).catch((e) => {
                    console.warn('登录后自动绑定邀请人失败:', (e && e.message) || e)
                })
            }
            // 拉取服务器购物车历史并合并到本地
            return fetchServerCart().then(() => bindWxOpenid().then(() => getUser()))
        })
}

// 服务器注册：向服务器注册会员账号（注册即会员），成功后自动绑定暂存的邀请码
function registerWithPhone(phone, code) {
    return api
        .request('/api/v1/auth/register', { method: 'POST', data: { phone, code }, noRetry: true })
        .then((res) => {
            setServerToken(res.token)
            // 注意：这里不把邀请码传给 applyServerUser，避免本地 invitedBy 被预填，
            // 否则 bindInviter 的“已绑定”拦截会让注册场景的自动绑定请求永远发不到服务器，
            // 导致邀请人端看不到「邀请好友 +1」、也收不到佣金。
            applyServerUser(res.user || {}, phone)
            // 注册成功后自动绑定邀请人（服务器持久化邀请关系，绑定成功后才写本地展示信息）
            const inviter = takePendingInviter()
            if (inviter) {
                bindInviter(inviter).catch((e) => {
                    console.warn('注册后自动绑定邀请人失败:', (e && e.message) || e)
                })
            }
            return fetchServerCart().then(() => bindWxOpenid().then(() => getUser()))
        })
}

// 退出登录：清除登录态、服务器 token 及当前账号的本地缓存
// （用户资料保留在 mall_user_<phone>，仅清除"当前活动账号"指针，避免多账号混淆；
//   服务器是唯一数据源，登录后重新拉取）
function logout() {
    const user = getUsers()
    const phone = String((user && user.phone) || '').trim()
    const key = (base) => (phone ? base + '_' + phone : base + '_guest')
    setServerToken('')
    wx.removeStorageSync(key(KEY_DEVICES))
    wx.removeStorageSync(key(KEY_DEVICE_MSGS))
    wx.removeStorageSync(key(KEY_CART))
    wx.removeStorageSync(key(KEY_ORDERS))
    wx.removeStorageSync(key(KEY_COMMISSION))
    wx.removeStorageSync(key(KEY_WITHDRAWALS))
    wx.removeStorageSync(key(KEY_ADDRESS))
    wx.removeStorageSync(key(KEY_SEARCH_HISTORY))
    serverInviteeCount = 0
    // 清除当前活动账号指针（该账号用户资料按手机号保留，下次登录按账号恢复）
    wx.removeStorageSync(KEY_ACTIVE_PHONE)
    return null
}

// ==================== 邀请关系 ====================
// 邀请关系已全部由服务器持久化：注册/登录时自动绑定邀请人（写入 users.invited_by），
// 邀请列表与邀请人数一律从服务器接口读取，本地不再维护/展示模拟团队成员。

// 绑定邀请码（服务端持久化邀请关系，仅首次绑定生效；成功后写本地展示信息）
function bindInviter(inviterCode) {
    inviterCode = String(inviterCode || '').trim().toUpperCase()
    const user = getUser()
    if (!user) return Promise.reject({ message: '请先登录' })
    if (!inviterCode) return Promise.reject({ message: '请输入邀请码' })
    if (inviterCode === myCode()) return Promise.reject({ message: '不能绑定自己的邀请码' })
    // 是否已绑定由服务器判定（本地 invitedBy 可能因历史 bug 残留而并非服务器真实状态）。
    // 服务器在已绑定时会返回“您已绑定邀请人…”，本地不提前拦截，保证绑定请求能到达服务器。
    return api
        .request('/api/v1/user/bind-inviter', {
            method: 'POST',
            data: { code: inviterCode },
            token: getServerToken(),
        })
        .then((su) => {
            updateUser({
                invitedBy: inviterCode,
                invitedByName: (su && su.invitedByName) || '好友',
            })
            // 佣金记录以服务器为准（fetchCommissions 拉取），本地不写入
            // 绑定成功：清除暂存的邀请码，避免下次登录/注册重复绑定
            wx.removeStorageSync(KEY_PENDING_INVITER)
            return { ok: true, msg: '已接受好友（' + ((su && su.invitedByName) || '好友') + '）的邀请' }
        })
        .catch((err) => {
            console.warn('绑定邀请码失败:', (err && err.message) || err)
            throw { message: (err && err.message) || '绑定失败' }
        })
}

// 我邀请的好友（服务器持久化的邀请关系，含消费/佣金统计）
let serverInviteeCount = 0 // 最近一次拉取的服务器真实被邀请人数（供 myInviteCount 等同步场景使用）
function fetchMyInvitees() {
    if (!isLoggedIn()) {
        serverInviteeCount = 0
        return Promise.resolve([])
    }
    return api
        .request('/api/v1/user/invitees', { token: getServerToken() })
        .then((data) => {
            serverInviteeCount = (data || []).length
            return data || []
        })
        .catch((err) => {
            console.warn('邀请列表拉取失败（请检查服务器是否可达/已登录）:', err && err.message)
            return []
        })
}

// 我的佣金记录（服务器：pending 待结算 / settled 已到账 / cancelled 已取消）
function fetchCommissions() {
    if (!isLoggedIn()) return Promise.resolve({ list: [], settleDays: SETTLE_DAYS })
    return api
        .request('/api/v1/user/commissions', { token: getServerToken() })
        .then((data) => data || { list: [], settleDays: SETTLE_DAYS })
        .catch((err) => {
            console.warn('佣金记录拉取失败（请检查服务器是否可达/已登录）:', err && err.message)
            return { list: [], settleDays: SETTLE_DAYS }
        })
}

// 无理由退款（支付后无理由退货期内可退；退款后订单关联的待结算佣金自动取消）
function refundOrder(orderId) {
    return api.request('/api/v1/orders/' + orderId + '/refund', {
        method: 'POST',
        token: getServerToken(),
        noRetry: true,
    })
}

// ==================== 购物车 ====================
function getCart() {
    return get(acctKey(KEY_CART), [])
}

// 属性签名：用于区分同商品不同规格的购物车项
function attrsKey(attrs) {
    if (!attrs || !attrs.length) return ''
    return attrs
        .map((a) => a.name + ':' + a.value)
        .sort()
        .join('|')
}

// 商品是否有必选属性
function productHasAttrs(product) {
    return !!(product && product.attributes && product.attributes.length)
}

// 商品属性是否已选全
function attrsComplete(product, attrs) {
    if (!productHasAttrs(product)) return true
    const selected = attrs || []
    return product.attributes.every((a) => selected.some((s) => s.name === a.name && s.value))
}

// 已选属性文本，如"版本:云台版 / 表带颜色:曜石黑"
function attrsText(attrs) {
    if (!attrs || !attrs.length) return ''
    return attrs.map((a) => a.name + ':' + a.value).join(' / ')
}

function addToCart(productId, count, attrs) {
    const cart = getCart()
    const key = attrsKey(attrs)
    const exist = cart.find((item) => item.productId === productId && attrsKey(item.attrs) === key)
    if (exist) {
        exist.count += count || 1
    } else {
        cart.unshift({
            id: genId(),
            productId,
            count: count || 1,
            checked: true,
            attrs: attrs || [],
        })
    }
    set(acctKey(KEY_CART), cart)
    syncCartToServer() // 登录用户把购物车历史同步到服务器（记录商品 id 与数量）
    return cart
}

function updateCartCount(cartId, count) {
    const cart = getCart()
    const item = cart.find((c) => c.id === cartId)
    if (item) item.count = Math.max(1, count)
    set(acctKey(KEY_CART), cart)
    syncCartToServer()
    return cart
}

function toggleCartChecked(cartId) {
    // 勾选状态是本地 UI 状态，不同步服务器（服务器只记录商品 id 与数量）
    const cart = getCart()
    const item = cart.find((c) => c.id === cartId)
    if (item) item.checked = !item.checked
    set(acctKey(KEY_CART), cart)
    return cart
}

function setCartCheckedAll(checked) {
    const cart = getCart().map((c) => Object.assign({}, c, { checked }))
    set(acctKey(KEY_CART), cart)
    return cart
}

function removeCartItems(cartIds) {
    const cart = getCart().filter((c) => cartIds.indexOf(c.id) === -1)
    set(acctKey(KEY_CART), cart)
    syncCartToServer()
    return cart
}

// 把本地购物车全量同步到服务器（仅登录用户；服务器只记录商品 id 与数量）
function syncCartToServer() {
    if (!isLoggedIn()) return Promise.resolve()
    const token = getServerToken()
    if (!token) return Promise.resolve()
    const items = getCart().map((c) => ({ productId: c.productId, quantity: c.count }))
    return api
        .request('/api/v1/cart/sync', { method: 'POST', data: { items }, token, noRetry: true })
        .catch((err) => {
            console.warn('购物车历史同步失败（本地数据保留）:', err && err.message)
        })
}

// 登录后拉取服务器购物车历史，与本地购物车合并（同商品数量取较大值），并回写服务器
function fetchServerCart() {
    if (!isLoggedIn()) return Promise.resolve([])
    const token = getServerToken()
    if (!token) return Promise.resolve([])
    return api
        .request('/api/v1/cart', { token, noRetry: true })
        .then((items) => {
            const list = items || []
            const serverMap = {}
            list.forEach((it) => {
                serverMap[it.productId] = it.quantity
            })
            const local = getCart()
            // 本地有、服务器没有的商品保留（并同步上去）
            const localOnly = local.filter((c) => !(c.productId in serverMap))
            // 服务器商品合并进本地（同商品数量取较大值，保留本地勾选态）
            const merged = localOnly.concat(
                list.map((it) => {
                    const existed = local.find((c) => c.productId === it.productId)
                    return {
                        id: existed ? existed.id : genId(),
                        productId: it.productId,
                        count: Math.max(it.quantity || 1, existed ? existed.count : 0),
                        checked: existed ? existed.checked : true,
                        attrs: existed ? existed.attrs || [] : [],
                    }
                })
            )
            set(acctKey(KEY_CART), merged)
            syncCartToServer() // 合并结果回写，保证两端一致
            return merged
        })
        .catch(() => getCart())
}

function getCartDetail() {
    return getCart().map((item) => {
        const product = getProductCache(item.productId)
        const attrsTextValue = attrsText(item.attrs)
        return Object.assign({}, item, {
            name: product ? product.name : '商品已下架',
            price: product ? product.price : 0,
            emoji: product ? product.emoji : '📦',
            colors: product ? product.colors : ['#ccc', '#999'],
            originalPrice: product ? product.originalPrice : 0,
            invalid: !product,
            service: product ? product.service : false,
            images: product ? product.images || [] : [],
            attrsText: attrsTextValue,
            // 商品有必选属性但未选（服务器恢复的购物车项可能缺失规格）
            missingSpec: productHasAttrs(product) && !attrsTextValue,
        })
    })
}

function calcCartTotal() {
    return getCartDetail().reduce((s, item) => {
        if (item.checked && !item.invalid) return s + item.price * item.count
        return s
    }, 0)
}

function calcCartCount() {
    return getCart().reduce((s, c) => s + c.count, 0)
}

// ==================== 订单 ====================
// 订单以服务器为准（fetchOrdersServer / fetchOrderServer 拉取），
// 本地仅缓存当前账号（按手机号隔离）的服务器数据，供离线兜底与展示加速。
// 本地下单/本地改状态等"伪造数据"路径已移除。

function getOrders(status) {
    const orders = get(acctKey(KEY_ORDERS), [])
    if (status && status !== 'all') return orders.filter((o) => o.status === status)
    return orders
}

function getOrder(id) {
    return get(acctKey(KEY_ORDERS), []).find((o) => o.id === id) || null
}


// ==================== 订单（服务器） ====================
// 服务器下单：价格以服务器商品表为准，地址/商品快照持久化，支付在服务器确认
function createOrderServer({ items, addressId, remark }) {
    const token = getServerToken()
    if (!token) return Promise.reject(new Error('未登录'))
    const data = {
        addressId,
        remark: remark || '',
        items: (items || []).map((i) => ({
            productId: i.productId || i.id,
            quantity: i.count || 1,
            attrs: i.attrs || [],
        })),
    }
    return api.request('/api/v1/orders', { method: 'POST', data, token }).then((order) => {
        const orders = get(acctKey(KEY_ORDERS), [])
        orders.unshift(order)
        set(acctKey(KEY_ORDERS), orders)
        return order
    })
}

// 服务器订单列表（账号数据：服务器为唯一数据源；仅未登录时返回空，登录但服务器不可达时兜底当前账号本地缓存）
function fetchOrdersServer(status) {
    if (!isLoggedIn()) return Promise.resolve([])
    const token = getServerToken()
    if (!token) return Promise.resolve([])
    return api
        .request('/api/v1/orders?status=' + (status || 'all'), { token })
        .then((list) => {
            const arr = Array.isArray(list) ? list : []
            set(acctKey(KEY_ORDERS), arr)
            return arr
        })
        .catch((err) => {
            console.warn('订单拉取失败（使用本地缓存）:', err && err.message)
            return getOrders(status)
        })
}

function fetchOrderServer(id) {
    if (!isLoggedIn()) return Promise.resolve(null)
    const token = getServerToken()
    if (!token) return Promise.resolve(null)
    return api
        .request('/api/v1/orders/' + id, { token })
        .then((order) => {
            const orders = get(acctKey(KEY_ORDERS), []).map((o) => (o.id === order.id ? order : o))
            set(acctKey(KEY_ORDERS), orders)
            return order
        })
        .catch((err) => {
            console.warn('订单详情拉取失败（使用本地缓存）:', err && err.message)
            return getOrder(id)
        })
}

function updateOrderServer(id, action) {
    const token = getServerToken()
    if (!token) return Promise.reject(new Error('未登录'))
    return api
        .request('/api/v1/orders/' + id + '/' + action, { method: 'POST', token, data: {} })
        .then((order) => {
            const orders = get(acctKey(KEY_ORDERS), []).map((o) => (o.id === order.id ? order : o))
            set(acctKey(KEY_ORDERS), orders)
            return order
        })
}

function cancelOrderServer(id) {
    return updateOrderServer(id, 'cancel')
}

function payOrderServer(id, payMethod) {
    const token = getServerToken()
    if (!token) return Promise.reject(new Error('未登录'))
    return api
        .request('/api/v1/orders/' + id + '/pay', {
            method: 'POST',
            token,
            data: { payMethod: payMethod || 'wechat' },
        })
        .then((res) => {
            // 真实微信支付：服务器返回 wx.requestPayment 参数（mode=real），订单由微信回调确认
            if (res && res.mode === 'real') return res
            // 模拟支付：服务器直接确认，本地更新订单缓存
            const order = res && res.order ? res.order : res
            const orders = get(acctKey(KEY_ORDERS), []).map((o) => (o.id === order.id ? order : o))
            set(acctKey(KEY_ORDERS), orders)
            return res
        })
}

function confirmOrderServer(id) {
    return updateOrderServer(id, 'confirm')
}

// ==================== 佣金 ====================
// 佣金记录以服务器为准（fetchCommissions 拉取，含 pending/settled/cancelled 状态），
// 本地不再写入/统计佣金（原 addCommission / getCommissionStat 等造假数据函数已移除）。

// ==================== 提现 ====================
// 提现记录以服务器为准（fetchWithdrawalsServer / applyWithdrawServer），
// 本地仅缓存当前账号（按手机号隔离）的服务器数据；本地提现等造假数据路径已移除。
function getWithdrawals() {
    return get(acctKey(KEY_WITHDRAWALS), [])
}

function methodName(method) {
    return { wechat: '微信零钱', alipay: '支付宝', bank: '银行卡' }[method] || method
}

// ==================== 提现（服务器） ====================
// 服务器事务内扣减余额，企业银行卡号（.env 配置）由服务器写入提现记录
function fetchWithdrawalsServer() {
    if (!isLoggedIn()) return Promise.resolve([])
    const token = getServerToken()
    if (!token) return Promise.resolve([])
    return api
        .request('/api/v1/withdrawals', { token })
        .then((list) => {
            const arr = Array.isArray(list) ? list : []
            set(acctKey(KEY_WITHDRAWALS), arr)
            return arr
        })
        .catch((err) => {
            console.warn('提现记录拉取失败（使用本地缓存）:', err && err.message)
            return getWithdrawals()
        })
}

function applyWithdrawServer({ amount, method, account }) {
    const token = getServerToken()
    if (!token) return Promise.reject(new Error('未登录'))
    return api
        .request('/api/v1/withdrawals', {
            method: 'POST',
            token,
            data: { amount: Number(amount) || 0, method, account: account || '' },
        })
        .then((res) => {
            const record = res && res.withdrawal ? res.withdrawal : res
            const list = getWithdrawals()
            list.unshift(record)
            set(acctKey(KEY_WITHDRAWALS), list)
            // 同步服务器余额到本地
            return api.request('/api/v1/user/me', { token }).then((su) => {
                updateUser({
                    balance: Number(su.balance) || 0,
                    totalCommission: Number(su.totalCommission) || 0,
                })
                return record
            })
        })
}

// ==================== 客服（服务器） ====================
function fetchSupportTickets() {
    const token = getServerToken()
    if (!token) return Promise.resolve([])
    return api.request('/api/v1/support/tickets', { token })
}

function createSupportTicket({ subject, productId, productName, message }) {
    const token = getServerToken()
    if (!token) return Promise.reject(new Error('未登录'))
    return api.request('/api/v1/support/tickets', {
        method: 'POST',
        token,
        data: { subject, productId: productId || '', productName: productName || '', message },
    })
}

function fetchSupportDetail(id) {
    const token = getServerToken()
    if (!token) return Promise.reject(new Error('未登录'))
    return api.request('/api/v1/support/tickets/' + id, { token })
}

function sendSupportMessage(id, content) {
    const token = getServerToken()
    if (!token) return Promise.reject(new Error('未登录'))
    return api.request('/api/v1/support/tickets/' + id + '/messages', {
        method: 'POST',
        token,
        data: { content },
    })
}

// 团队客服成员：我的客服收件箱（团队服务会话分配给我）
function fetchSupportInbox() {
    const token = getServerToken()
    if (!token) return Promise.resolve([])
    return api.request('/api/v1/support/inbox', { token })
}

// 团队客服成员：回复（后端校验是否为该会话分配客服）
function replySupportTicket(id, content) {
    const token = getServerToken()
    if (!token) return Promise.reject(new Error('未登录'))
    return api.request('/api/v1/support/tickets/' + id + '/reply', {
        method: 'POST',
        token,
        data: { content },
    })
}

// 团队客服成员：关闭会话
function closeSupportTicket(id) {
    const token = getServerToken()
    if (!token) return Promise.reject(new Error('未登录'))
    return api.request('/api/v1/support/tickets/' + id + '/close', { method: 'POST', token, data: {} })
}

// 团长为团队指定客服成员（接收/回复团队服务客服会话）
function setTeamSupportMember(memberPhone) {
    const token = getServerToken()
    if (!token) return Promise.reject(new Error('未登录'))
    return api.request('/api/v1/team/support-member', {
        method: 'POST',
        token,
        data: { memberPhone },
    })
}


// ==================== 团队 ====================
// 建团资格：邀请人数 > 2，或所在团队经营金额 > 1w
const TEAM_MIN_INVITES = 3 // 邀请人数 > 2（即 ≥3）
const TEAM_MIN_BUSINESS = 10000 // 经营金额 > 1w

// 我的团队（服务器）
function getMyTeam() {
    if (!isLoggedIn()) return Promise.resolve(null)
    return api
        .request('/api/v1/team/my', { token: getServerToken(), noRetry: true })
        .then((data) => data || null)
        .catch(() => null)
}

// ==================== 团队金库 ====================
// 团队服务订单 90% 分成入金库，仅团长可提取/向成员转账

// 团长提取金库到我的余额
function treasuryWithdraw(amount) {
    return api.request('/api/v1/team/treasury/withdraw', {
        method: 'POST',
        data: { amount: Number(amount) || 0 },
        token: getServerToken(),
        noRetry: true,
    })
}

// 团长从金库向团队成员余额转账
function treasuryTransfer(phone, amount) {
    return api.request('/api/v1/team/treasury/transfer', {
        method: 'POST',
        data: { phone, amount: Number(amount) || 0 },
        token: getServerToken(),
        noRetry: true,
    })
}

// 我的团队金库流水
function fetchTreasuryLogs() {
    if (!isLoggedIn()) return Promise.resolve([])
    return api
        .request('/api/v1/team/treasury/logs', { token: getServerToken(), noRetry: true })
        .then((data) => data || [])
        .catch(() => [])
}

// 金库流水类型文案
function treasuryLogText(log) {
    if (!log) return ''
    const t = {
        income: '服务分成收入（90%）',
        withdraw: '团长提取到余额',
        transfer: '团长向成员转账',
    }[log.type] || log.type
    return t
}

// 我的邀请人数（服务器持久化的真实被邀请人数；进入团队页/个人中心时先 fetchMyInvitees 一次）
function myInviteCount() {
    return serverInviteeCount
}

// 是否具备建团资格（邀请人数 > 2 或 所在团队经营 >1w；邀请人数以服务器统计为准）
function canCreateTeam(myTeam) {
    if (myInviteCount() > 2) return true
    if (myTeam && Number(myTeam.businessAmount) > TEAM_MIN_BUSINESS) return true
    return false
}

// 创建团队（建团资格由服务器依据真实邀请关系校验）
function createTeam(name) {
    return api.request('/api/v1/team/create', {
        method: 'POST',
        data: {
            name,
            businessAmount: 0,
        },
        token: getServerToken(),
        noRetry: true,
    })
}

// 加入团队成为成员（需不在任何团队中）
function joinTeam(teamId) {
    return api.request('/api/v1/team/join', {
        method: 'POST',
        data: { teamId },
        token: getServerToken(),
        noRetry: true,
    })
}

// ==================== 团队邀请（团长邀请 → 对方同意后入团） ====================
// 邀请候选：我邀请的好友（附团队/待处理状态，供优先选取）
function fetchTeamInviteCandidates() {
    if (!isLoggedIn()) return Promise.resolve([])
    return api
        .request('/api/v1/team/invites/candidates', { token: getServerToken() })
        .then((d) => d || [])
        .catch((err) => {
            console.warn('邀请候选拉取失败:', (err && err.message) || err)
            return []
        })
}

// 我收到的团队邀请（同意后入团）
function fetchTeamInviteInbox() {
    if (!isLoggedIn()) return Promise.resolve([])
    return api
        .request('/api/v1/team/invites/inbox', { token: getServerToken() })
        .then((d) => d || [])
        .catch((err) => {
            console.warn('团队邀请收件箱拉取失败:', (err && err.message) || err)
            return []
        })
}

// 我发出的邀请（团长出件箱，含历史状态）
function fetchTeamInviteOutbox() {
    if (!isLoggedIn()) return Promise.resolve([])
    return api
        .request('/api/v1/team/invites/outbox', { token: getServerToken() })
        .then((d) => d || [])
        .catch((err) => {
            console.warn('团队邀请出件箱拉取失败:', (err && err.message) || err)
            return []
        })
}

// 邀请好友/用户入团（仅团长；对方同意后入团）
function inviteToTeam(phone) {
    if (!isLoggedIn()) return Promise.reject({ message: '请先登录' })
    return api.request('/api/v1/team/invites', {
        method: 'POST',
        data: { phone },
        token: getServerToken(),
    })
}

// 接受团队邀请并入团
function acceptTeamInvite(id) {
    if (!isLoggedIn()) return Promise.reject({ message: '请先登录' })
    return api.request('/api/v1/team/invites/' + id + '/accept', {
        method: 'POST',
        data: {},
        token: getServerToken(),
    })
}

// 拒绝团队邀请
function rejectTeamInvite(id) {
    if (!isLoggedIn()) return Promise.reject({ message: '请先登录' })
    return api.request('/api/v1/team/invites/' + id + '/reject', {
        method: 'POST',
        data: {},
        token: getServerToken(),
    })
}

// 团长取消待处理邀请
function cancelTeamInvite(id) {
    if (!isLoggedIn()) return Promise.reject({ message: '请先登录' })
    return api.request('/api/v1/team/invites/' + id + '/cancel', {
        method: 'POST',
        data: {},
        token: getServerToken(),
    })
}

// 手机号查询用户（邀请入团；返回是否已注册/已在团队）
function searchUserByPhone(phone) {
    if (!isLoggedIn()) return Promise.reject({ message: '请先登录' })
    return api.request('/api/v1/users/search?phone=' + String(phone || '').trim(), { token: getServerToken() })
}

// 团队发布服务类商品（类目需为服务大类下二级类目）
function publishServiceProduct(form) {
    return api.request('/api/v1/team/products', {
        method: 'POST',
        data: form,
        token: getServerToken(),
        noRetry: true,
    })
}

// 服务类商品成交后累计来源团队经营金额（按服务来源团队名）
function recordTeamBusiness(amount, sourceTeam) {
    if (!isLoggedIn() || !amount || !sourceTeam) return Promise.resolve()
    return api
        .request('/api/v1/team/business', {
            method: 'POST',
            data: { amount, teamName: sourceTeam },
            token: getServerToken(),
            noRetry: true,
        })
        .catch(() => {})
}

// 服务大类下二级类目（团队发布服务可选）
function getServiceLevel2Categories() {
    return getLevel2Categories('service')
}

// 团员申请创建新团（需现任团长审核）
function applyCreateTeam(name) {
    return api.request('/api/v1/team/apply-create', {
        method: 'POST',
        data: {
            name,
            businessAmount: 0,
        },
        token: getServerToken(),
        noRetry: true,
    })
}

// 我提交的建团申请
function getMyTeamRequests() {
    if (!isLoggedIn()) return Promise.resolve([])
    return api
        .request('/api/v1/team/requests/my', { token: getServerToken(), noRetry: true })
        .then((data) => data || [])
        .catch(() => [])
}

// 待我审核的建团申请（团长收件箱）
function getTeamRequestInbox() {
    if (!isLoggedIn()) return Promise.resolve([])
    return api
        .request('/api/v1/team/requests/inbox', { token: getServerToken(), noRetry: true })
        .then((data) => data || [])
        .catch(() => [])
}

// 审核通过建团申请
function approveTeamRequest(id) {
    return api.request('/api/v1/team/requests/' + id + '/approve', {
        method: 'POST',
        data: {},
        token: getServerToken(),
        noRetry: true,
    })
}

// 驳回建团申请
function rejectTeamRequest(id) {
    return api.request('/api/v1/team/requests/' + id + '/reject', {
        method: 'POST',
        data: {},
        token: getServerToken(),
        noRetry: true,
    })
}

// ==================== 设备 ====================
const KEY_DEVICES = 'mall_devices'
const KEY_DEVICE_MSGS = 'mall_device_messages'

const DEVICE_TYPES = [
    { type: 'camera', name: '智能摄像头', icon: '📹', colors: ['#3B82F6', '#1E40AF'] },
    { type: 'smoke', name: '烟雾报警器', icon: '🔥', colors: ['#F97316', '#C2410C'] },
    { type: 'gas', name: '燃气报警器', icon: '🔥', colors: ['#EF4444', '#991B1B'] },
    { type: 'sos', name: '一键求助', icon: '🆘', colors: ['#F43F5E', '#9F1239'] },
    { type: 'fall', name: '跌倒监测手环', icon: '⌚', colors: ['#8B5CF6', '#5B21B6'] },
    { type: 'watch', name: '定位手表', icon: '⌚', colors: ['#06B6D4', '#155E75'] },
    { type: 'lock', name: '智能门锁', icon: '🔒', colors: ['#475569', '#1E293B'] },
    { type: 'infrared', name: '红外感应器', icon: '🚨', colors: ['#F59E0B', '#B45309'] },
    { type: 'water', name: '水浸传感器', icon: '💧', colors: ['#0EA5E9', '#075985'] },
]

function getDeviceType(type) {
    return DEVICE_TYPES.find((t) => t.type === type) || DEVICE_TYPES[0]
}

// 设备与账号绑定：未登录（未注册）时一律不返回设备数据
function getDevices() {
    if (!isLoggedIn()) return []
    const devices = get(acctKey(KEY_DEVICES), [])
    return devices.map((d) => Object.assign({}, d, getDeviceType(d.type)))
}

function getDevice(id) {
    if (!isLoggedIn()) return null
    const d = get(acctKey(KEY_DEVICES), []).find((x) => x.id === id)
    if (!d) return null
    return Object.assign({}, d, getDeviceType(d.type))
}

function getDevicesStat() {
    const devices = getDevices()
    return {
        total: devices.length,
        online: devices.filter((d) => d.status === 'online').length,
        alarm: devices.filter((d) => d.status === 'alarm').length,
        offline: devices.filter((d) => d.status === 'offline').length,
    }
}

// ---------- 服务器读写（设备/消息是账号数据，以服务器为准，本地仅作当前账号缓存） ----------

// 拉取服务器设备列表并写入本地缓存
function fetchDevices() {
    if (!isLoggedIn()) return Promise.resolve([])
    const token = getServerToken()
    if (!token) return Promise.resolve([])
    return api
        .request('/api/v1/devices', { token })
        .then((list) => {
            const devices = (list || []).map((d) => Object.assign({}, d, getDeviceType(d.type)))
            set(acctKey(KEY_DEVICES), devices)
            return devices
        })
        .catch((e) => {
            console.warn('拉取设备列表失败，使用本地缓存:', (e && e.message) || e)
            return getDevices()
        })
}

// 拉取服务器设备消息并写入本地缓存；deviceId 缺省返回全部
function fetchDeviceMessages(deviceId) {
    if (!isLoggedIn()) return Promise.resolve([])
    const token = getServerToken()
    if (!token) return Promise.resolve([])
    return api
        .request('/api/v1/messages', { token })
        .then((list) => {
            const all = (list || []).map((m) => Object.assign({}, m, { deviceId: m.deviceId || '' }))
            set(acctKey(KEY_DEVICE_MSGS), all)
            return deviceId ? all.filter((m) => m.deviceId === deviceId) : all
        })
        .catch((e) => {
            console.warn('拉取设备消息失败，使用本地缓存:', (e && e.message) || e)
            return getDeviceMessages(deviceId)
        })
}

// 绑定硬件平台设备（服务器向平台校验IMEI获取UID；UID全局唯一，一台硬件只归属一个用户）
function bindDevice({ name, type, imei }) {
    if (!isLoggedIn()) return Promise.reject({ message: '请先登录' })
    const token = getServerToken()
    return api
        .request('/api/v1/devices/bind', {
            method: 'POST',
            data: { name, type, imei },
            token,
        })
        .then((device) => {
            const devices = getDevices()
            devices.unshift(Object.assign({}, device, getDeviceType(device.type)))
            set(acctKey(KEY_DEVICES), devices)
            return Object.assign({}, device, getDeviceType(device.type))
        })
        .catch((e) => {
            console.warn('绑定设备失败:', (e && e.message) || e)
            throw { message: (e && e.message) || '绑定失败' }
        })
}

// 移除设备（服务器删除 + 本地缓存同步）
function removeDevice(id) {
    if (!isLoggedIn()) return Promise.reject({ message: '请先登录' })
    const token = getServerToken()
    return api
        .request('/api/v1/devices/' + id, { method: 'DELETE', token })
        .then(() => {
            const devices = getDevices().filter((d) => d.id !== id)
            set(acctKey(KEY_DEVICES), devices)
            const msgs = getDeviceMessages().filter((m) => m.deviceId !== id)
            set(acctKey(KEY_DEVICE_MSGS), msgs)
            return devices
        })
        .catch((e) => {
            console.warn('移除设备失败:', (e && e.message) || e)
            throw { message: (e && e.message) || '移除失败' }
        })
}

// 更新设备（名称/通知开关）：先改本地再同步服务器；服务器失败时保留本地修改
function updateDevice(id, patch) {
    const before = getDevices()
    const local = before.map((d) => (d.id === id ? Object.assign({}, d, patch) : d))
    set(acctKey(KEY_DEVICES), local)
    if (!isLoggedIn()) return Promise.resolve(local.find((d) => d.id === id))
    const token = getServerToken()
    return api
        .request('/api/v1/devices/' + id, { method: 'PUT', data: patch, token })
        .then((device) => {
            set(acctKey(KEY_DEVICES), getDevices().map((d) => (d.id === id ? Object.assign({}, d, device) : d)))
            return device
        })
        .catch((e) => {
            console.warn('更新设备失败，已保留本地修改:', (e && e.message) || e)
            return local.find((d) => d.id === id)
        })
}

// 触发设备报警（服务器生成消息 + 微信订阅消息推送）
function triggerAlarm(deviceId) {
    if (!isLoggedIn()) return Promise.reject({ message: '请先登录' })
    const token = getServerToken()
    return api
        .request('/api/v1/devices/' + deviceId + '/alarm', { method: 'POST', data: {}, token })
        .then((res) => {
            const m = res.message || {}
            const alarm = {
                title: m.title || '设备报警',
                content: m.content || '设备检测到异常情况，请及时处理',
            }
            updateDevice(deviceId, { status: 'alarm', lastActive: Date.now() })
            return { device: getDevice(deviceId), msg: m, alarm, push: res.push }
        })
        .catch((e) => {
            console.warn('触发设备报警失败:', (e && e.message) || e)
            throw { message: (e && e.message) || '报警触发失败' }
        })
}

// 模拟设备报警（演示）：走服务器报警接口
function simulateAlarm(deviceId) {
    return triggerAlarm(deviceId)
}

function getDeviceMessages(deviceId) {
    // 设备消息与账号绑定：未登录不返回；消息以服务器为准（fetchDeviceMessages 拉取）
    if (!isLoggedIn()) return []
    const all = get(acctKey(KEY_DEVICE_MSGS), [])
    return deviceId ? all.filter((m) => m.deviceId === deviceId) : all
}

// 标记已读：本地即时生效 + 异步同步服务器
function markDeviceMsgRead(id) {
    if (isLoggedIn()) {
        api
            .request('/api/v1/messages/' + id + '/read', { method: 'POST', data: {}, token: getServerToken() })
            .catch(() => {})
    }
    const list = getDeviceMessages().map((m) => (m.id === id ? Object.assign({}, m, { read: true }) : m))
    set(acctKey(KEY_DEVICE_MSGS), list)
    return list
}

function markAllDeviceMsgRead() {
    const list = getDeviceMessages().map((m) => Object.assign({}, m, { read: true }))
    set(acctKey(KEY_DEVICE_MSGS), list)
    return list
}

function getUnreadDeviceMsgCount() {
    return getDeviceMessages().filter((m) => !m.read).length
}

// ==================== 地址 ====================
// 地址绑定账号保存在服务器（SQLite 持久化），本地仅缓存当前账号（按手机号隔离）数据
function getAddresses() {
    return get(acctKey(KEY_ADDRESS), [])
}

// ==================== 地址（服务器同步） ====================
// 地址绑定账号保存在服务器（SQLite 持久化；默认地址唯一）
function syncAddressesFromServer() {
    if (!isLoggedIn()) return Promise.resolve([])
    const token = getServerToken()
    if (!token) return Promise.resolve([])
    return api
        .request('/api/v1/addresses', { token })
        .then((list) => {
            const arr = Array.isArray(list) ? list : []
            set(acctKey(KEY_ADDRESS), arr)
            return arr
        })
        .catch((err) => {
            console.warn('地址拉取失败（使用本地缓存）:', err && err.message)
            return getAddresses()
        })
}

function saveAddressServer(address) {
    const token = getServerToken()
    if (!token) return Promise.reject(new Error('未登录'))
    const payload = {
        name: address.name,
        phone: address.phone,
        region: address.region,
        detail: address.detail,
        isDefault: !!address.isDefault,
    }
    const path = address.id ? '/api/v1/addresses/' + address.id : '/api/v1/addresses'
    const method = address.id ? 'PUT' : 'POST'
    return api.request(path, { method, data: payload, token }).then((saved) => {
        // 更新本地缓存（默认唯一）
        const list = getAddresses().filter((a) => a.id !== saved.id)
        list.forEach((a) => {
            if (saved.isDefault) a.isDefault = false
        })
        list.unshift(saved)
        set(acctKey(KEY_ADDRESS), list)
        return saved
    })
}

function deleteAddressServer(id) {
    const token = getServerToken()
    if (!token) return Promise.reject(new Error('未登录'))
    return api
        .request('/api/v1/addresses/' + id, { method: 'DELETE', token })
        .then(() => {
            let list = getAddresses().filter((a) => a.id !== id)
            if (list.length && !list.some((a) => a.isDefault)) list[0].isDefault = true
            set(acctKey(KEY_ADDRESS), list)
            return list
        })
}

function setDefaultAddressServer(id) {
    const token = getServerToken()
    if (!token) return Promise.reject(new Error('未登录'))
    return api
        .request('/api/v1/addresses/' + id + '/default', { method: 'POST', token })
        .then(() => {
            const list = getAddresses().map((a) => Object.assign({}, a, { isDefault: a.id === id }))
            set(acctKey(KEY_ADDRESS), list)
            return list
        })
}


// 地址以服务器为准（syncAddressesFromServer / saveAddressServer 等），
// 本地仅保留缓存读取；本地保存/删除地址等造假数据路径已移除。
function getDefaultAddress() {
    const list = getAddresses()
    return list.find((a) => a.isDefault) || list[0] || null
}

module.exports = {
    // 常量
    COMMISSION_RATE,
    WITHDRAW_MIN,
    WITHDRAW_FEE,
    RATE_TEXT,
    SMS_CODE,
    // 工具
    formatMoney,
    genOrderNo,
    // 商品（服务器数据缓存）
    initProductCache,
    fetchProductsOnce,
    getCategoriesCache,
    getLevel1Categories,
    getLevel2Categories,
    getProductsCache,
    getProductCache,
    getProductsByCategoryCache,
    searchProductsCache,
    // 搜索历史
    getSearchHistory,
    saveSearchHistory,
    removeSearchHistory,
    clearSearchHistory,
    // 服务器登录同步
    getServerToken,
    setServerToken,
    syncServerLogin,
    bindWxOpenid,
    saveInviter,
    takePendingInviter,
    // 用户
    getUser,
    initUser,
    updateUser,
    setUserProfile,
    updateUserProfile,
    DEFAULT_AVATAR,
    isFirstEnter,
    myCode,
    // 登录 / 注册
    isLoggedIn,
    requireLogin,
    loginWithPhone,
    registerWithPhone,
    logout,
    // 邀请
    bindInviter,
    fetchMyInvitees,
    fetchCommissions,
    refundOrder,
    // 团队
    getMyTeam,
    treasuryWithdraw,
    treasuryTransfer,
    fetchTreasuryLogs,
    treasuryLogText,
    myInviteCount,
    canCreateTeam,
    createTeam,
    joinTeam,
    // 团队邀请
    fetchTeamInviteCandidates,
    fetchTeamInviteInbox,
    fetchTeamInviteOutbox,
    inviteToTeam,
    acceptTeamInvite,
    rejectTeamInvite,
    cancelTeamInvite,
    searchUserByPhone,
    publishServiceProduct,
    recordTeamBusiness,
    getServiceLevel2Categories,
    applyCreateTeam,
    getMyTeamRequests,
    getTeamRequestInbox,
    approveTeamRequest,
    rejectTeamRequest,
    TEAM_MIN_INVITES,
    TEAM_MIN_BUSINESS,
    // 购物车
    getCart,
    addToCart,
    updateCartCount,
    toggleCartChecked,
    setCartCheckedAll,
    removeCartItems,
    getCartDetail,
    calcCartTotal,
    calcCartCount,
    // 商品属性
    attrsKey,
    attrsText,
    attrsComplete,
    productHasAttrs,
    // 购物车历史（服务器同步）
    syncCartToServer,
    fetchServerCart,
    // 订单（服务器为准；本地 getOrders/getOrder 仅读当前账号缓存）
    getOrders,
    getOrder,
    // 订单（服务器）
    createOrderServer,
    fetchOrdersServer,
    fetchOrderServer,
    cancelOrderServer,
    payOrderServer,
    confirmOrderServer,
    // 提现
    getWithdrawals,
    methodName,
    // 提现（服务器）
    fetchWithdrawalsServer,
    applyWithdrawServer,
    // 设备
    DEVICE_TYPES,
    getDevices,
    getDevice,
    getDevicesStat,
    fetchDevices,
    fetchDeviceMessages,
    bindDevice,
    removeDevice,
    updateDevice,
    getDeviceMessages,
    markDeviceMsgRead,
    markAllDeviceMsgRead,
    getUnreadDeviceMsgCount,
    triggerAlarm,
    simulateAlarm,
    // 地址
    getAddresses,
    getDefaultAddress,
    // 地址（服务器）
    syncAddressesFromServer,
    saveAddressServer,
    deleteAddressServer,
    setDefaultAddressServer,
    // 客服（服务器）
    fetchSupportTickets,
    createSupportTicket,
    fetchSupportDetail,
    sendSupportMessage,
    fetchSupportInbox,
    replySupportTicket,
    closeSupportTicket,
    setTeamSupportMember,
}
