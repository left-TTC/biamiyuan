// pages/deviceAdd/deviceAdd.js
const store = require('../../utils/store.js')

Page({
    data: {
        types: store.DEVICE_TYPES,
        selectedType: 'camera',
        name: '',
        imei: '',
        saving: false,
    },

    selectType(e) {
        this.setData({ selectedType: e.currentTarget.dataset.type })
    },

    onName(e) {
        this.setData({ name: e.detail.value })
    },

    onImei(e) {
        this.setData({ imei: e.detail.value })
    },

    // 生成 15 位演示 IMEI（86 开头）；正式环境请填写设备包装/机身标签上的真实 IMEI
    genImei() {
        let imei = '86'
        for (let i = 0; i < 13; i++) imei += Math.floor(Math.random() * 10)
        this.setData({ imei })
    },

    save() {
        // 设备与账号绑定：需登录后才能添加设备
        if (!store.requireLogin()) return
        const name = this.data.name.trim()
        const imei = this.data.imei.trim()
        if (!name) {
            wx.showToast({ title: '请输入设备名称', icon: 'none' })
            return
        }
        if (!imei) {
            wx.showToast({ title: '请输入设备IMEI', icon: 'none' })
            return
        }
        if (!/^\d{15}$/.test(imei) && !/^[A-Za-z0-9-]{6,30}$/.test(imei)) {
            wx.showToast({ title: 'IMEI 为 15 位数字（或 SN 6-30 位）', icon: 'none' })
            return
        }
        this.setData({ saving: true })
        // 绑定走服务器：校验 IMEI、获取平台 UID（一台硬件只归属一个用户）
        store
            .bindDevice({ name, type: this.data.selectedType, imei })
            .then(() => {
                wx.showToast({ title: '设备绑定成功', icon: 'success' })
                setTimeout(() => wx.navigateBack(), 600)
            })
            .catch((e) => {
                this.setData({ saving: false })
                wx.showToast({ title: (e && e.message) || '绑定失败', icon: 'none' })
            })
    },
})
