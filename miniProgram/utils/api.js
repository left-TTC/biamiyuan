// utils/api.js
// 服务器请求封装

// ==================== 服务器地址（手写固定） ====================
// 服务器地址为「手写固定地址」：服务器 IP 变更时，直接手动编辑下方 BASE_URL 即可，
// 无自动切换/记忆机制。
//
// 注意事项：
//   - 真机调试/预览时不能写 127.0.0.1（那是手机自身），必须写电脑的局域网 IP，
//     且手机与电脑连接同一 Wi-Fi。查看 IP：macOS 终端运行 ipconfig getifaddr en0
//   - 局域网 IP 会随 WiFi/路由器变化；若小程序连不上服务器：
//       1) 终端运行 ipconfig getifaddr en0 获取当前 IP；
//       2) 将下方 BASE_URL 更新为当前 IP。
const BASE_URL = 'http://192.168.1.194:8080'

/**
 * send request
 * @param {string} path 接口路径，如 /api/v1/products
 * @param {object} options { method, data, token, retried }
 * @returns {Promise<any>} 解析后的 data 字段
 */
function request(path, options = {}) {
    const { method = 'GET', data = {}, token, retried = false, noRetry = false } = options
    const useToken = token || wx.getStorageSync('mall_server_token') || ''
    return new Promise((resolve, reject) => {
        // 统一响应处理（401 自动重登 + 业务码判断）
        function handleRes(res) {
            const body = res.data
            // 401：token 失效（如服务器重启），自动重新登录后重试一次
            // noRetry：登录/注册/验证码等认证接口不做自动重登，避免绕过验证码校验
            if (res.statusCode === 401 && !retried && !noRetry) {
                reLogin()
                    .then((newToken) => {
                        if (!newToken) {
                            reject(new Error((body && body.msg) || '登录已失效，请重新登录'))
                            return
                        }
                        request(path, { method, data, token: newToken, retried: true })
                            .then(resolve)
                            .catch(reject)
                    })
                    .catch(() => reject(new Error((body && body.msg) || '登录已失效')))
                return
            }
            if (res.statusCode >= 200 && res.statusCode < 300 && body && body.code === 0) {
                resolve(body.data)
            } else {
                reject(new Error((body && body.msg) || '请求失败'))
            }
        }
        wx.request({
            url: BASE_URL + path,
            method,
            data,
            header: {
                'Content-Type': 'application/json',
                ...(useToken ? { Authorization: 'Bearer ' + useToken } : {}),
            },
            success: handleRes,
            fail: (err) => {
                console.log('Request fail: ', err)
                reject(err)
            },
        })
    })
}

// 服务器 token 失效时：用当前活动账号手机号重新登录，刷新 mall_server_token
// （用户资料按账号隔离存储为 mall_user_<phone>，这里读取会话指针 mall_active_phone）
function reLogin() {
    const phone = wx.getStorageSync('mall_active_phone') || ''
    if (!phone) return Promise.resolve('')
    return request('/api/v1/auth/code', { method: 'POST', data: { phone }, noRetry: true })
        .then(() => request('/api/v1/auth/login', { method: 'POST', data: { phone, code: '12345' }, noRetry: true }))
        .then((res) => {
            if (res && res.token) {
                wx.setStorageSync('mall_server_token', res.token)
                return res.token
            }
            return ''
        })
        .catch(() => '')
}

// 上传文件（multipart）到服务器
// @param {string} path 接口路径，如 /api/v1/user/avatar
// @param {string} filePath 本地临时文件路径
// @param {object} opts { name='file', formData, token }
// @returns {Promise<any>} 解析后的 data 字段（如 { url, name }）
function upload(path, filePath, opts = {}) {
    const { name = 'file', formData = {}, token } = opts
    const useToken = token || wx.getStorageSync('mall_server_token') || ''
    return new Promise((resolve, reject) => {
        wx.uploadFile({
            url: BASE_URL + path,
            filePath,
            name,
            formData,
            header: {
                ...(useToken ? { Authorization: 'Bearer ' + useToken } : {}),
            },
            success(res) {
                let body = {}
                try {
                    body = JSON.parse(res.data)
                } catch (e) {
                    // 非 JSON 响应
                }
                if (res.statusCode >= 200 && res.statusCode < 300 && body && body.code === 0) {
                    resolve(body.data)
                } else {
                    reject(new Error((body && body.msg) || '上传失败'))
                }
            },
            fail(err) {
                console.log('Upload fail: ', err)
                reject(err)
            },
        })
    })
}

module.exports = {
    BASE_URL,
    request,
    upload,
}
