// pages/support/support.js
// 客服会话列表：我的会话 + 新建会话（普通商品→后台客服，团队服务→团队指定客服成员）
const store = require('../../utils/store.js')
const util = require('../../utils/util.js')

Page({
    data: {
        productId: '',
        productName: '',
        sourceTeam: '',
        tickets: [],
        inbox: [], // 我是团队客服时的收件箱
        showForm: false,
        subject: '',
        message: '',
    },

    onLoad(options) {
        this.setData({
            productId: options.productId || '',
            productName: options.productName || '',
            sourceTeam: options.sourceTeam || '',
        })
    },

    onShow() {
        if (!store.requireLogin()) return
        this.loadTickets()
    },

    loadTickets() {
        store.fetchSupportTickets().then((list) => {
            this.setData({
                tickets: (list || []).map((t) =>
                    Object.assign({}, t, {
                        statusText: t.status === 'open' ? '待处理' : '已关闭',
                        timeText: util.formatTime(new Date(t.lastTime || t.createdAt)),
                    })
                ),
            })
        })
        // 若我是某团队指定客服，展示分配给我的收件箱
        store.fetchSupportInbox().then((list) => {
            this.setData({
                inbox: (list || []).map((t) =>
                    Object.assign({}, t, {
                        statusText: t.status === 'open' ? '待处理' : '已关闭',
                        timeText: util.formatTime(new Date(t.lastTime || t.createdAt)),
                    })
                ),
            })
        })
    },

    openTicket(e) {
        wx.navigateTo({ url: '/pages/supportChat/supportChat?id=' + e.currentTarget.dataset.id })
    },

    toggleForm() {
        this.setData({ showForm: !this.data.showForm })
    },

    onSubject(e) {
        this.setData({ subject: e.detail.value })
    },

    onMessage(e) {
        this.setData({ message: e.detail.value })
    },

    submit() {
        const subject = this.data.subject.trim()
        const message = this.data.message.trim()
        if (!subject) return wx.showToast({ title: '请填写问题标题', icon: 'none' })
        if (!message) return wx.showToast({ title: '请填写问题描述', icon: 'none' })
        store
            .createSupportTicket({
                subject,
                productId: this.data.productId,
                productName: this.data.productName,
                message,
            })
            .then((t) => {
                this.setData({ showForm: false, subject: '', message: '' })
                wx.navigateTo({ url: '/pages/supportChat/supportChat?id=' + t.id })
            })
            .catch((err) => {
                wx.showToast({ title: (err && err.message) || '提交失败', icon: 'none' })
            })
    },
})