// pages/address/address.js
const store = require('../../utils/store.js')

Page({
    data: {
        addresses: [],
        mode: '', // select 为选择模式
    },

    onLoad(options) {
        this.setData({ mode: options.mode || '' })
    },

    onShow() {
        // 地址绑定账号保存在服务器，进入时同步
        store.syncAddressesFromServer().then((list) => {
            this.setData({ addresses: list || [] })
        })
    },

    addAddress() {
        wx.navigateTo({ url: '/pages/addressEdit/addressEdit' })
    },

    editAddress(e) {
        wx.navigateTo({ url: '/pages/addressEdit/addressEdit?id=' + e.currentTarget.dataset.id })
    },

    selectAddress(e) {
        if (this.data.mode === 'select') {
            wx.setStorageSync('selected_address_id', e.currentTarget.dataset.id)
            wx.navigateBack()
        }
    },

    setDefault(e) {
        const id = e.currentTarget.dataset.id
        store.setDefaultAddressServer(id).then((list) => {
            this.setData({ addresses: list || [] })
        })
    },

    deleteAddress(e) {
        const id = e.currentTarget.dataset.id
        wx.showModal({
            title: '提示',
            content: '确定删除该地址吗？',
            success: (res) => {
                if (res.confirm) {
                    store.deleteAddressServer(id).then((list) => {
                        this.setData({ addresses: list || [] })
                    })
                }
            },
        })
    },
})
