// pages/profile/profile.js
// 编辑资料：头像 + 昵称（微信官方头像昵称填写能力）
// 头像：chooseAvatar 临时文件 → 上传服务器 uploads/（失败则 wx.saveFile 本地持久化）
// 昵称：input type="nickname" 微信原生昵称填写
const store = require('../../utils/store.js')
const api = require('../../utils/api.js')

Page({
    data: {
        nickName: '',
        avatarUrl: '',
        phone: '',
        savingAvatar: false,
    },

    onLoad() {
        if (!store.requireLogin()) return
        const user = store.getUser()
        if (!user) return
        this.setData({
            nickName: user.nickName || '',
            avatarUrl: user.avatarUrl || store.DEFAULT_AVATAR,
            phone: user.phone || '',
        })
    },

    // 选择微信头像（临时文件路径），立即预览并上传服务器
    onChooseAvatar(e) {
        const temp = e.detail && e.detail.avatarUrl
        if (!temp) return
        this.setData({ avatarUrl: temp, savingAvatar: true })
        api
            .upload('/api/v1/user/avatar', temp)
            .then((res) => {
                // 上传成功：使用服务器 URL（http://.../uploads/avatar_xxx.jpg）
                const url = res && res.url ? api.BASE_URL + res.url : temp
                this.setData({ avatarUrl: url, savingAvatar: false })
            })
            .catch(() => {
                // 服务器不可用：wx.saveFile 保存为本地持久文件（重启后仍可用）
                wx.saveFile({
                    tempFilePath: temp,
                    success: (r) => this.setData({ avatarUrl: r.savedFilePath, savingAvatar: false }),
                    fail: () => this.setData({ avatarUrl: temp, savingAvatar: false }),
                })
            })
    },

    onNickInput(e) {
        this.setData({ nickName: e.detail.value })
    },

    save() {
        if (this.data.savingAvatar) {
            wx.showToast({ title: '头像上传中，请稍候', icon: 'none' })
            return
        }
        const nickName = (this.data.nickName || '').trim()
        if (!nickName) {
            wx.showToast({ title: '请输入昵称', icon: 'none' })
            return
        }
        store.updateUserProfile({
            nickName,
            avatarUrl: this.data.avatarUrl === store.DEFAULT_AVATAR ? '' : this.data.avatarUrl,
        })
        wx.showToast({ title: '资料已保存', icon: 'success' })
        setTimeout(() => wx.navigateBack(), 600)
    },
})
