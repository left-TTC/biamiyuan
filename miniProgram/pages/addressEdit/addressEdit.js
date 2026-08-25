// pages/addressEdit/addressEdit.js
const store = require('../../utils/store.js')

Page({
    data: {
        id: '',
        name: '',
        phone: '',
        region: '',
        detail: '',
        isDefault: false,
    },

    onLoad(options) {
        if (options.id) {
            // 以服务器地址为准（绑定账号保存在服务器）
            store.syncAddressesFromServer().then((list) => {
                const address = (list || []).find((a) => a.id === options.id)
                if (address) {
                    this.setData({
                        id: address.id,
                        name: address.name,
                        phone: address.phone,
                        region: address.region,
                        detail: address.detail,
                        isDefault: !!address.isDefault,
                    })
                }
            })
        }
    },

    onName(e) {
        this.setData({ name: e.detail.value })
    },

    onPhone(e) {
        this.setData({ phone: e.detail.value })
    },

    onRegion(e) {
        this.setData({ region: e.detail.value })
    },

    onDetail(e) {
        this.setData({ detail: e.detail.value })
    },

    toggleDefault(e) {
        this.setData({ isDefault: e.detail.value })
    },

    save() {
        const { name, phone, region, detail, isDefault } = this.data
        if (!name.trim()) return wx.showToast({ title: '请填写收货人', icon: 'none' })
        if (!/^1\d{10}$/.test(phone.trim())) return wx.showToast({ title: '请填写正确的手机号', icon: 'none' })
        if (!region.trim()) return wx.showToast({ title: '请填写所在地区', icon: 'none' })
        if (!detail.trim()) return wx.showToast({ title: '请填写详细地址', icon: 'none' })
        // 地址绑定账号保存在服务器（默认地址唯一，服务器自动处理）
        store
            .saveAddressServer({
                id: this.data.id || undefined,
                name: name.trim(),
                phone: phone.trim(),
                region: region.trim(),
                detail: detail.trim(),
                isDefault,
            })
            .then(() => {
                wx.showToast({ title: '保存成功', icon: 'success' })
                setTimeout(() => wx.navigateBack(), 500)
            })
            .catch((err) => {
                wx.showToast({ title: (err && err.message) || '保存失败', icon: 'none' })
            })
    },
})
