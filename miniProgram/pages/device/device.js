// pages/device/device.js
const store = require('../../utils/store.js')
const util = require('../../utils/util.js')

const STATUS_TEXT = { online: '在线', offline: '离线', alarm: '报警中' }

Page({
    data: {
        device: null,
        messages: [],
    },

    onLoad(options) {
        this.deviceId = options.id
    },

    onShow() {
        // 设备与账号绑定：需登录后查看设备
        if (!store.requireLogin()) return
        this.load()
    },

    load() {
        // 设备/消息以服务器为准（硬件平台推送、报警均写入服务器）
        Promise.all([store.fetchDevices(), store.fetchDeviceMessages(this.deviceId)]).then(() => {
            const device = store.getDevice(this.deviceId)
            if (!device) {
                wx.showToast({ title: '设备不存在', icon: 'none' })
                setTimeout(() => wx.navigateBack(), 800)
                return
            }
            const messages = store.getDeviceMessages(device.id).map((m) =>
                Object.assign({}, m, {
                    timeText: util.formatTime(new Date(m.time)),
                })
            )
            this.setData({
                device: Object.assign({}, device, {
                    statusText: STATUS_TEXT[device.status] || device.status,
                    lastActiveText: util.formatTime(new Date(device.lastActive)),
                    createTimeText: util.formatTime(new Date(device.createTime)),
                    healthText: formatHealth(device.lastHealth),
                }),
                messages,
            })
        })
    },

    // 设备通知开关（服务器持久化；开启后平台报警会推送微信订阅消息）
    toggleNotify() {
        const device = store.getDevice(this.deviceId)
        if (!device) return
        const enabled = !device.notifyEnabled
        store.updateDevice(this.deviceId, { notifyEnabled: enabled }).then(() => {
            wx.showToast({ title: enabled ? '已开启该设备通知' : '已关闭该设备通知', icon: 'none' })
            this.load()
        })
    },

    // 演示：模拟设备报警（服务器生成消息 + 微信订阅消息推送）
    simulateAlarm() {
        store
            .simulateAlarm(this.deviceId)
            .then((res) => {
                const pushMsg = (res.push && res.push.msg) || ''
                wx.showModal({
                    title: '🚨 ' + res.alarm.title,
                    content: res.alarm.content + '\n\n' + pushMsg,
                    showCancel: false,
                    success: () => this.load(),
                })
            })
            .catch((e) => {
                wx.showToast({ title: (e && e.message) || '报警触发失败', icon: 'none' })
            })
    },

    removeDevice() {
        wx.showModal({
            title: '提示',
            content: '确定移除该设备吗？相关消息记录也将删除',
            success: (res) => {
                if (res.confirm) {
                    store.removeDevice(this.deviceId).then(() => {
                        wx.showToast({ title: '已移除设备', icon: 'success' })
                        setTimeout(() => wx.navigateBack(), 600)
                    })
                }
            },
        })
    },
})

// 最近健康数据 → 展示文本（心率/血氧/体温/血压/血糖）
function formatHealth(h) {
    if (!h) return ''
    const parts = []
    if (h.heartRate) parts.push('心率 ' + h.heartRate + ' 次/分')
    if (h.bloodOxygen) parts.push('血氧 ' + h.bloodOxygen + '%')
    if (h.temperature) parts.push('体温 ' + Number(h.temperature).toFixed(1) + '℃')
    if (h.bpHigh && h.bpLow) parts.push('血压 ' + h.bpHigh + '/' + h.bpLow)
    if (h.bloodSugar) parts.push('血糖 ' + Number(h.bloodSugar).toFixed(1))
    return parts.join('、')
}
