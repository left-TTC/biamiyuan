// pages/team/team.js
const store = require('../../utils/store.js')

// 邀请状态文案
function inviteStatusText(s) {
    return { pending: '待确认', accepted: '已入团', rejected: '已拒绝', cancelled: '已取消' }[s] || s
}

Page({
    data: {
        // 团队信息
        myTeam: null,
        inviteCount: 0, // 建团资格判断用（邀请人数 > 2）
        canCreate: false,
        createName: '',
        creating: false,
        // 团员申请建新团（团长审核）
        isLeader: false,
        myApply: null,
        myApplyList: [],
        applyName: '',
        applyStatusText: '',
        applying: false,
        inbox: [],
        joinTeamId: '',
        joining: false,
        // 邀请成员入团（团长邀请 → 对方同意后入团）
        showInvite: false,
        invitePhone: '',
        inviteSearching: false,
        searchUserResult: null,
        inviteCandidates: [],
        teamInviteOutbox: [],
        invitingPhone: '',
        teamInviteInbox: [], // 我收到的团队邀请（无团队时展示）
        // 发布服务
        showPublish: false,
        publishBusy: false,
        serviceCats: [],
        pubForm: {
            name: '',
            price: '',
            desc: '',
            emoji: '🛠',
            category: '',
            attributes: [],
        },
        // 我的团队发布的服务商品
        teamProducts: [],
        // 指定客服成员（团长）
        showSupportMember: false,
        supportMemberPhone: '',
        // 团队金库（服务分成 90% 入金库，仅团长可支配）
        showTreasury: false,
        showTreasuryWithdraw: false,
        showTreasuryTransfer: false,
        treasuryWithdrawAmount: '',
        transferPhone: '',
        transferAmount: '',
        treasuryLogs: [],
        treasuryBusy: false,
    },

    onShow() {
        if (!store.requireLogin()) return
        this.refresh()
    },

    refresh() {
        const user = store.getUser()
        // 邀请人数 = 服务器持久化的真实被邀请人数（先拉取一次，供显示与建团资格判断）
        store
            .fetchMyInvitees()
            .then(() => {
                this.setData({ inviteCount: store.myInviteCount() })
                return store.getMyTeam()
            })
            .then((myTeam) => {
                const isLeader = !!(myTeam && user && myTeam.ownerPhone === user.phone)
                this.setData({
                    myTeam,
                    isLeader,
                    canCreate: store.canCreateTeam(myTeam),
                    serviceCats: store.getServiceLevel2Categories(),
                })
                // 我收到的团队邀请（无团队时展示；同意后入团）
                store.fetchTeamInviteInbox().then((list) => {
                    const inbox = (list || []).map((x) =>
                        Object.assign({}, x, {
                            timeText: new Date(x.createdAt).toLocaleString('zh-CN'),
                        })
                    )
                    this.setData({ teamInviteInbox: inbox })
                })
                // 团长：邀请候选（优先邀请好友）与已发邀请
                if (isLeader) {
                    store.fetchTeamInviteCandidates().then((cands) => {
                        const candidates = (cands || []).map((c) =>
                            Object.assign({}, c, {
                                statusText: c.inMyTeam
                                    ? '已是成员'
                                    : c.pending
                                        ? '已邀请'
                                        : c.inTeam
                                            ? '已在「' + (c.teamName || '其他团队') + '」'
                                            : '',
                            })
                        )
                        this.setData({ inviteCandidates: candidates })
                    })
                    store.fetchTeamInviteOutbox().then((list) => {
                        const outbox = (list || []).map((x) =>
                            Object.assign({}, x, {
                                statusText: inviteStatusText(x.status),
                                timeText: new Date(x.createdAt).toLocaleString('zh-CN'),
                            })
                        )
                        this.setData({ teamInviteOutbox: outbox })
                    })
                }
            // 我的团队发布的服务商品
            if (myTeam) {
                const teamProducts = store
                    .getProductsCache()
                    .filter((p) => p.service && p.sourceTeam === myTeam.name)
                this.setData({ teamProducts })
            }
            // 团队金库流水
            if (myTeam) {
                store.fetchTreasuryLogs().then((logs) => {
                    const list = (logs || []).map((l) =>
                        Object.assign({}, l, {
                            text: store.treasuryLogText(l),
                            sign: l.type === 'income' ? '+' : '-',
                            timeText: new Date(l.createdAt).toLocaleString('zh-CN'),
                        })
                    )
                    this.setData({ treasuryLogs: list })
                })
            }
            // 我提交的建团申请
            store.getMyTeamRequests().then((reqs) => {
                const pending = reqs.find((r) => r.status === 'pending')
                const myApply = pending || reqs[0] || null
                let applyStatusText = ''
                if (myApply) {
                    if (myApply.status === 'pending') {
                        applyStatusText = '⏳ 申请审核中，请耐心等待'
                    } else if (myApply.status === 'approved') {
                        applyStatusText = '✅ 申请已通过，你已是「' + myApply.teamName + '」的团长'
                    } else {
                        applyStatusText = '❌ 申请已被驳回，可重新申请'
                    }
                }
                this.setData({ myApply, myApplyList: reqs, applyStatusText })
            })
            // 团长收件箱
            if (isLeader) {
                store.getTeamRequestInbox().then((inbox) => {
                    const list = inbox.map((r) =>
                        Object.assign({}, r, {
                            timeText: new Date(r.createdAt).toLocaleString('zh-CN'),
                        })
                    )
                    this.setData({ inbox: list })
                })
            }
        })
    },

    // ---------- 创建团队 ----------
    onCreateName(e) {
        this.setData({ createName: e.detail.value })
    },

    createTeam() {
        const name = (this.data.createName || '').trim()
        if (!name) {
            wx.showToast({ title: '请输入团队名称', icon: 'none' })
            return
        }
        this.setData({ creating: true })
        store
            .createTeam(name)
            .then((team) => {
                wx.showToast({ title: '团队创建成功', icon: 'success' })
                this.setData({ createName: '', myTeam: team, canCreate: false })
                this.refresh()
            })
            .catch((err) => {
                wx.showToast({ title: err.message || '创建失败', icon: 'none' })
            })
            .finally(() => this.setData({ creating: false }))
    },

    // ---------- 加入团队 ----------
    onJoinTeamId(e) {
        this.setData({ joinTeamId: e.detail.value })
    },

    joinTeam() {
        const teamId = (this.data.joinTeamId || '').trim()
        if (!teamId) {
            wx.showToast({ title: '请输入团队 ID', icon: 'none' })
            return
        }
        this.setData({ joining: true })
        store
            .joinTeam(teamId)
            .then((team) => {
                wx.showToast({ title: '已加入团队「' + team.name + '」', icon: 'success' })
                this.setData({ joinTeamId: '' })
                this.refresh()
            })
            .catch((err) => {
                wx.showToast({ title: err.message || '加入失败', icon: 'none' })
            })
            .finally(() => this.setData({ joining: false }))
    },

    // ---------- 邀请成员入团（团长邀请 → 对方同意后入团） ----------
    openInvite() {
        const myTeam = this.data.myTeam
        if (!myTeam) return
        this.setData({ showInvite: true, invitePhone: '', searchUserResult: null })
        // 打开弹层时刷新候选与出件箱
        store.fetchTeamInviteCandidates().then((cands) => {
            const candidates = (cands || []).map((c) =>
                Object.assign({}, c, {
                    statusText: c.inMyTeam
                        ? '已是成员'
                        : c.pending
                            ? '已邀请'
                            : c.inTeam
                                ? '已在「' + (c.teamName || '其他团队') + '」'
                                : '',
                })
            )
            this.setData({ inviteCandidates: candidates })
        })
        store.fetchTeamInviteOutbox().then((list) => {
            const outbox = (list || []).map((x) =>
                Object.assign({}, x, {
                    statusText: inviteStatusText(x.status),
                    timeText: new Date(x.createdAt).toLocaleString('zh-CN'),
                })
            )
            this.setData({ teamInviteOutbox: outbox })
        })
    },

    closeInvite() {
        this.setData({ showInvite: false })
    },

    onInvitePhone(e) {
        this.setData({ invitePhone: e.detail.value })
    },

    // 手机号查询用户（邀请入团）
    doSearchUser() {
        const phone = (this.data.invitePhone || '').trim()
        if (!/^1\d{10}$/.test(phone)) {
            wx.showToast({ title: '请填写正确的手机号', icon: 'none' })
            return
        }
        this.setData({ inviteSearching: true })
        store
            .searchUserByPhone(phone)
            .then((u) => {
                this.setData({
                    searchUserResult: Object.assign({}, u, {
                        statusText: u.inMyTeam
                            ? '已是成员'
                            : u.pending
                                ? '已邀请，等待对方确认'
                                : u.inTeam
                                    ? '已在团队「' + (u.teamName || '') + '」'
                                    : '',
                    }),
                })
            })
            .catch((err) => {
                this.setData({ searchUserResult: null })
                wx.showToast({ title: (err && err.message) || '查询失败', icon: 'none' })
            })
            .finally(() => this.setData({ inviteSearching: false }))
    },

    // 发起邀请（好友候选 / 手机号查询结果共用）
    inviteUser(e) {
        const phone = e.currentTarget.dataset.phone
        if (this.data.invitingPhone) return
        this.setData({ invitingPhone: phone })
        store
            .inviteToTeam(phone)
            .then(() => {
                wx.showToast({ title: '已发送邀请', icon: 'success' })
                store.fetchTeamInviteCandidates().then((cands) => {
                    this.setData({
                        inviteCandidates: (cands || []).map((c) =>
                            Object.assign({}, c, {
                                statusText: c.inMyTeam
                                    ? '已是成员'
                                    : c.pending
                                        ? '已邀请'
                                        : c.inTeam
                                            ? '已在「' + (c.teamName || '其他团队') + '」'
                                            : '',
                            })
                        ),
                    })
                })
                store.fetchTeamInviteOutbox().then((list) => {
                    this.setData({
                        teamInviteOutbox: (list || []).map((x) =>
                            Object.assign({}, x, {
                                statusText: inviteStatusText(x.status),
                                timeText: new Date(x.createdAt).toLocaleString('zh-CN'),
                            })
                        ),
                    })
                })
                const sr = this.data.searchUserResult
                if (sr && sr.phone === phone) {
                    this.setData({ searchUserResult: Object.assign({}, sr, { pending: true, statusText: '已邀请，等待对方确认' }) })
                }
            })
            .catch((err) => {
                wx.showToast({ title: (err && err.message) || '邀请失败', icon: 'none' })
            })
            .finally(() => this.setData({ invitingPhone: '' }))
    },

    // 团长取消待处理邀请
    cancelInvite(e) {
        const id = e.currentTarget.dataset.id
        wx.showModal({
            title: '取消邀请',
            content: '确认取消该邀请？',
            success: (res) => {
                if (!res.confirm) return
                store
                    .cancelTeamInvite(id)
                    .then(() => {
                        wx.showToast({ title: '已取消邀请', icon: 'none' })
                        store.fetchTeamInviteOutbox().then((list) => {
                            this.setData({
                                teamInviteOutbox: (list || []).map((x) =>
                                    Object.assign({}, x, {
                                        statusText: inviteStatusText(x.status),
                                        timeText: new Date(x.createdAt).toLocaleString('zh-CN'),
                                    })
                                ),
                            })
                        })
                    })
                    .catch((err) => wx.showToast({ title: (err && err.message) || '取消失败', icon: 'none' }))
            },
        })
    },

    // 我收到的团队邀请：同意入团
    acceptInvite(e) {
        const id = e.currentTarget.dataset.id
        const item = this.data.teamInviteInbox.find((x) => x.id === id)
        wx.showModal({
            title: '接受邀请',
            content: '确认加入团队「' + (item ? item.teamName : '') + '」？加入后将在团队中作为成员。',
            success: (res) => {
                if (!res.confirm) return
                store
                    .acceptTeamInvite(id)
                    .then(() => {
                        wx.showToast({ title: '已加入团队', icon: 'success' })
                        this.refresh()
                    })
                    .catch((err) => {
                        wx.showToast({ title: (err && err.message) || '操作失败', icon: 'none' })
                    })
            },
        })
    },

    // 我收到的团队邀请：拒绝
    rejectInvite(e) {
        const id = e.currentTarget.dataset.id
        wx.showModal({
            title: '拒绝邀请',
            content: '确认拒绝该团队邀请？',
            success: (res) => {
                if (!res.confirm) return
                store
                    .rejectTeamInvite(id)
                    .then(() => {
                        wx.showToast({ title: '已拒绝', icon: 'none' })
                        this.refresh()
                    })
                    .catch((err) => {
                        wx.showToast({ title: (err && err.message) || '操作失败', icon: 'none' })
                    })
            },
        })
    },

    // ---------- 团员申请创建新团（团长审核） ----------
    onApplyName(e) {
        this.setData({ applyName: e.detail.value })
    },

    applyCreate() {
        const name = (this.data.applyName || '').trim()
        if (!name) {
            wx.showToast({ title: '请输入新团队名称', icon: 'none' })
            return
        }
        this.setData({ applying: true })
        store
            .applyCreateTeam(name)
            .then((reqObj) => {
                wx.showToast({ title: '申请已提交，等待团长审核', icon: 'success' })
                this.setData({ applyName: '', myApply: reqObj })
                this.refresh()
            })
            .catch((err) => {
                wx.showToast({ title: err.message || '提交失败', icon: 'none' })
            })
            .finally(() => this.setData({ applying: false }))
    },

    // ---------- 团长审核 ----------
    approveReq(e) {
        const id = e.currentTarget.dataset.id
        wx.showModal({
            title: '审核建团申请',
            content: '确认通过该申请？通过后成员将创建自己的团队并离开当前团队',
            success: (res) => {
                if (!res.confirm) return
                store
                    .approveTeamRequest(id)
                    .then(() => {
                        wx.showToast({ title: '已通过，新团队已创建', icon: 'success' })
                        this.refresh()
                    })
                    .catch((err) => wx.showToast({ title: err.message || '操作失败', icon: 'none' }))
            },
        })
    },

    rejectReq(e) {
        const id = e.currentTarget.dataset.id
        wx.showModal({
            title: '审核建团申请',
            content: '确认驳回该建团申请？',
            success: (res) => {
                if (!res.confirm) return
                store
                    .rejectTeamRequest(id)
                    .then(() => {
                        wx.showToast({ title: '已驳回', icon: 'none' })
                        this.refresh()
                    })
                    .catch((err) => wx.showToast({ title: err.message || '操作失败', icon: 'none' }))
            },
        })
    },

    // ---------- 指定客服成员（团队服务会话由该成员接收/回复） ----------
    openSupportMember() {
        const myTeam = this.data.myTeam
        if (!myTeam) return
        this.setData({
            showSupportMember: true,
            supportMemberPhone: myTeam.supportMemberPhone || '',
        })
    },

    closeSupportMember() {
        this.setData({ showSupportMember: false })
    },

    onSupportMember(e) {
        this.setData({ supportMemberPhone: e.currentTarget.dataset.phone })
    },

    saveSupportMember() {
        const phone = this.data.supportMemberPhone
        if (!phone) {
            wx.showToast({ title: '请选择客服成员', icon: 'none' })
            return
        }
        store
            .setTeamSupportMember(phone)
            .then(() => {
                this.setData({ showSupportMember: false })
                wx.showToast({ title: '客服成员已指定', icon: 'success' })
                this.refresh()
            })
            .catch((err) => {
                wx.showToast({ title: (err && err.message) || '设置失败', icon: 'none' })
            })
    },

    // ---------- 团队金库（团长提取 / 向成员转账） ----------
    openTreasury() {
        this.setData({ showTreasury: true })
    },

    closeTreasury() {
        this.setData({
            showTreasury: false,
            showTreasuryWithdraw: false,
            showTreasuryTransfer: false,
        })
    },

    openTreasuryWithdraw() {
        this.setData({
            showTreasuryWithdraw: true,
            showTreasuryTransfer: false,
            treasuryWithdrawAmount: '',
        })
    },

    openTreasuryTransfer() {
        this.setData({
            showTreasuryTransfer: true,
            showTreasuryWithdraw: false,
            transferPhone: '',
            transferAmount: '',
        })
    },

    onWithdrawAmount(e) {
        this.setData({ treasuryWithdrawAmount: e.detail.value })
    },

    onTransferPhone(e) {
        this.setData({ transferPhone: e.detail.value })
    },

    onTransferAmount(e) {
        this.setData({ transferAmount: e.detail.value })
    },

    doTreasuryWithdraw() {
        const amount = Number(this.data.treasuryWithdrawAmount)
        if (!amount || amount <= 0) {
            wx.showToast({ title: '请输入提取金额', icon: 'none' })
            return
        }
        this.setData({ treasuryBusy: true })
        store
            .treasuryWithdraw(amount)
            .then(() => {
                wx.showToast({ title: '已提取到我的余额', icon: 'success' })
                this.setData({ showTreasuryWithdraw: false, treasuryWithdrawAmount: '' })
                this.refresh()
            })
            .catch((err) => {
                wx.showToast({ title: (err && err.message) || '提取失败', icon: 'none' })
            })
            .finally(() => this.setData({ treasuryBusy: false }))
    },

    doTreasuryTransfer() {
        const phone = (this.data.transferPhone || '').trim()
        const amount = Number(this.data.transferAmount)
        if (!phone || !/^1\d{10}$/.test(phone)) {
            wx.showToast({ title: '请填写正确的成员手机号', icon: 'none' })
            return
        }
        if (!amount || amount <= 0) {
            wx.showToast({ title: '请输入转账金额', icon: 'none' })
            return
        }
        wx.showModal({
            title: '金库转账',
            content: '从团队金库向 ' + phone + ' 转账 ¥' + amount + ' ？',
            success: (res) => {
                if (!res.confirm) return
                this.setData({ treasuryBusy: true })
                store
                    .treasuryTransfer(phone, amount)
                    .then(() => {
                        wx.showToast({ title: '转账成功', icon: 'success' })
                        this.setData({ showTreasuryTransfer: false, transferPhone: '', transferAmount: '' })
                        this.refresh()
                    })
                    .catch((err) => {
                        wx.showToast({ title: (err && err.message) || '转账失败', icon: 'none' })
                    })
                    .finally(() => this.setData({ treasuryBusy: false }))
            },
        })
    },

    // ---------- 发布服务 ----------
    openPublish() {
        const cats = store.getServiceLevel2Categories()
        this.setData({
            showPublish: true,
            serviceCats: cats,
            pubForm: {
                name: '',
                price: '',
                desc: '',
                emoji: '🛠',
                category: cats.length ? cats[0].id : '',
                attributes: [],
            },
        })
    },

    closePublish() {
        this.setData({ showPublish: false })
    },

    noop() {},

    onPubField(e) {
        const { field } = e.currentTarget.dataset
        this.setData({ ['pubForm.' + field]: e.detail.value })
    },

    onPubCategory(e) {
        this.setData({ 'pubForm.category': e.currentTarget.dataset.id })
    },

    onPubAttr(e) {
        const { index, field } = e.currentTarget.dataset
        const attrs = this.data.pubForm.attributes.slice()
        if (!attrs[index]) attrs[index] = { name: '', values: '' }
        attrs[index][field] = e.detail.value
        this.setData({ 'pubForm.attributes': attrs })
    },

    addPubAttr() {
        const attrs = this.data.pubForm.attributes.slice()
        attrs.push({ name: '', values: '' })
        this.setData({ 'pubForm.attributes': attrs })
    },

    removePubAttr(e) {
        const index = Number(e.currentTarget.dataset.index)
        const attrs = this.data.pubForm.attributes.filter((_, i) => i !== index)
        this.setData({ 'pubForm.attributes': attrs })
    },

    publishService() {
        const f = this.data.pubForm
        const price = Number(f.price)
        if (!f.name.trim() || !price || price <= 0) {
            wx.showToast({ title: '请填写服务名称与价格', icon: 'none' })
            return
        }
        if (!f.category) {
            wx.showToast({ title: '请选择服务类目', icon: 'none' })
            return
        }
        const attributes = f.attributes
            .filter((a) => a.name && a.name.trim())
            .map((a) => ({
                name: a.name.trim(),
                values: String(a.values || '')
                    .split(/[,，]/)
                    .map((v) => v.trim())
                    .filter(Boolean),
            }))
            .filter((a) => a.values.length)
        this.setData({ publishBusy: true })
        store
            .publishServiceProduct({
                name: f.name.trim(),
                price,
                originalPrice: Number(f.price),
                desc: (f.desc || '').trim(),
                emoji: f.emoji || '🛠',
                category: f.category,
                attributes,
            })
            .then((product) => {
                this.setData({ showPublish: false })
                wx.showToast({ title: '服务已发布', icon: 'success' })
                // 刷新商品缓存，让新服务在商城服务类目下可见
                store
                    .fetchProductsOnce(true)
                    .then(() => this.refresh())
                    .catch(() => this.refresh())
            })
            .catch((err) => {
                wx.showToast({ title: err.message || '发布失败', icon: 'none' })
            })
            .finally(() => this.setData({ publishBusy: false }))
    },

    // 查看我邀请的好友（去邀请页）
    goInvite() {
        wx.navigateTo({ url: '/pages/invite/invite' })
    },

    // 去商城服务类目
    goServiceMall() {
        wx.switchTab({ url: '/pages/category/category' })
    },
})
