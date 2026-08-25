// pages/supportChat/supportChat.js
// 客服会话详情：用户发消息；团队客服成员可回复/关闭分配给他的会话
// 实时性：发送后立即本地回显 + 定时轮询拉取最新消息（客服回复自动出现）
const store = require('../../utils/store.js')
const util = require('../../utils/util.js')

const POLL_INTERVAL = 3000 // 轮询间隔（毫秒）

Page({
    data: {
        id: '',
        ticket: null,
        messages: [],
        input: '',
        isOwner: false, // 我是会话归属用户
        isAssignee: false, // 我是该会话团队客服
        scrollIntoView: '', // 触发 scroll-view 滚动到底部（每次重新赋值才会滚动）
    },

    onLoad(options) {
        this.setData({ id: options.id || '' })
        this.load()
    },

    onShow() {
        if (this.data.id) this.load()
        this.startPolling()
    },

    onHide() {
        this.stopPolling()
    },

    onUnload() {
        this.stopPolling()
        clearTimeout(this._scrollReset)
    },

    // ---------- 消息加载与渲染 ----------
    decorate(list) {
        return (list || []).map((m) =>
            Object.assign({}, m, {
                timeText: util.formatTime(new Date(m.createdAt)),
            })
        )
    },

    // 渲染消息并滚动到底部；silent 模式（轮询）仅在消息有变化时更新，避免列表闪烁
    render(msgs, silent) {
        const list = this.decorate(msgs)
        const lastId = list.length ? list[list.length - 1].id : ''
        const curLastId = this.data.messages.length ? this.data.messages[this.data.messages.length - 1].id : ''
        if (silent && lastId === curLastId) return
        this.setData({ messages: list, scrollIntoView: 'chat-bottom' })
        const last = list[list.length - 1]
        if (last && this.data.ticket) {
            this.setData({ 'ticket.lastMessage': last.content })
        }
        // 清除 scrollIntoView，使下次能再次触发滚动
        clearTimeout(this._scrollReset)
        this._scrollReset = setTimeout(() => {
            if (this.data.scrollIntoView) this.setData({ scrollIntoView: '' })
        }, 500)
    },

    load() {
        store
            .fetchSupportDetail(this.data.id)
            .then((d) => {
                if (!d || !d.ticket) {
                    wx.showToast({ title: '会话不存在', icon: 'none' })
                    return
                }
                const user = store.getUser() || {}
                const t = d.ticket
                this.setData({
                    ticket: t,
                    isOwner: t.userId === user.id,
                    isAssignee: t.assigneeType === 'team' && t.assigneePhone === user.phone,
                })
                this.render(d.messages || [])
            })
            .catch((err) => {
                wx.showToast({ title: (err && err.message) || '加载失败', icon: 'none' })
            })
    },

    // ---------- 定时轮询：客服回复后自动出现在聊天框 ----------
    startPolling() {
        this.stopPolling()
        this._pollTimer = setInterval(() => {
            if (!this.data.id) return
            store
                .fetchSupportDetail(this.data.id)
                .then((d) => {
                    if (!d || !d.ticket) return
                    // 状态变化（如被关闭）时同步
                    if (this.data.ticket && d.ticket.status !== this.data.ticket.status) {
                        this.setData({ 'ticket.status': d.ticket.status })
                    }
                    this.render(d.messages || [], true)
                })
                .catch(() => {}) // 轮询静默失败，等待下一轮
        }, POLL_INTERVAL)
    },

    stopPolling() {
        if (this._pollTimer) {
            clearInterval(this._pollTimer)
            this._pollTimer = null
        }
    },

    onInput(e) {
        this.setData({ input: e.detail.value })
    },

    send() {
        const content = this.data.input.trim()
        if (!content) return
        if (!this.data.ticket) return
        if (this.data.ticket.status !== 'open') return
        const id = this.data.id
        const doSend = this.data.isOwner
            ? store.sendSupportMessage(id, content)
            : this.data.isAssignee
              ? store.replySupportTicket(id, content)
              : Promise.reject(new Error('无权发送消息'))
        doSend
            .then((msg) => {
                this.setData({ input: '' })
                // 本地立即回显发送的消息，无需等待服务器重新拉取
                const sent = msg ? [msg] : []
                this.render(this.data.messages.concat(sent))
            })
            .catch((err) => {
                wx.showToast({ title: (err && err.message) || '发送失败', icon: 'none' })
            })
    },

    close() {
        wx.showModal({
            title: '提示',
            content: '确定关闭该会话吗？',
            success: (res) => {
                if (res.confirm) {
                    store
                        .closeSupportTicket(this.data.id)
                        .then(() => {
                            wx.showToast({ title: '会话已关闭', icon: 'success' })
                            setTimeout(() => wx.navigateBack(), 500)
                        })
                        .catch((err) => {
                            wx.showToast({ title: (err && err.message) || '关闭失败', icon: 'none' })
                        })
                }
            },
        })
    },
})