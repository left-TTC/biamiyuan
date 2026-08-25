// pages/devices/devices.js
const store = require('../../utils/store.js')
const util = require('../../utils/util.js')

const STATUS_TEXT = { online: '在线', offline: '离线', alarm: '报警中' }

Page({
    data: {
        devices: [],
        stat: { total: 0, online: 0, alarm: 0, offline: 0 },
        loading: true,
    },

    onShow() {
        // 设备与账号绑定：需登录后查看/管理设备
        if (!store.requireLogin()) return
        this.refresh()
    },

    refresh() {
        this.setData({ loading: true })
        // 设备与消息均以服务器为准（硬件平台推送写入服务器）
        store.fetchDevices().then((devices) => {
            const list = devices.map((d) =>
                Object.assign({}, d, {
                    statusText: STATUS_TEXT[d.status] || d.status,
                    lastActiveText: util.formatTime(new Date(d.lastActive)),
                    locationText: d.lastLocText || '',
                })
            )
            this.setData({
                devices: list,
                stat: store.getDevicesStat(),
                loading: false,
            })
        })
    },

    onPullDownRefresh() {
        this.refresh()
        wx.stopPullDownRefresh()
    },

    goDevice(e) {
        wx.navigateTo({ url: '/pages/device/device?id=' + e.currentTarget.dataset.id })
    },

    goAdd() {
        wx.navigateTo({ url: '/pages/deviceAdd/deviceAdd' })
    },
})
