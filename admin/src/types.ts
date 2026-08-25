// 数据类型（与 Go 服务器返回结构一致）

export interface User {
    id: string
    phone: string
    nickName: string
    avatarUrl?: string
    role: string
    balance: number
    totalCommission: number
    promoterCode: string
    notifyEnabled: boolean
    openid?: string
    createdAt: number
}

export interface Device {
    id: string
    userId: string
    name: string
    type: string
    typeName?: string
    sn: string
    status: 'online' | 'offline' | 'alarm' | string
    battery: number
    notifyEnabled: boolean
    lastActive: number
    createTime: number
}

export interface Message {
    id: string
    deviceId: string
    type: 'alarm' | 'status' | 'info' | string
    title: string
    content: string
    time: number
    read: boolean
}

export interface ProductAttribute {
    name: string
    values: string[]
}

export interface Product {
    id: string
    name: string
    desc: string
    price: number
    originalPrice: number
    emoji: string
    colors: string[]
    images: string[]
    sales: number
    category: string
    tags: string[]
    detail: string[]
    attributes?: ProductAttribute[] // 内置属性（创建者定义，下单时需选择）
    service?: boolean // 服务类商品（服务大类下）
    sourceTeam?: string // 服务来源（团队名/官方）
}

export interface Stats {
    users: number
    devices: number
    messages: number
    products: number
    categories: number
    alarms: number
}

export interface ProductEdit {
    id: string
    productId: string
    field: string
    oldValue: string
    newValue: string
    operator: string
    createdAt: number
}

export interface DBStatus {
    connected: boolean
    driver: string
    path: string
    tables: string[]
    productEdits: number
    recentEdits: ProductEdit[]
}

export interface Category {
    id: string
    name: string
    parentId: string // 空 = 一级类目；非空 = 所属一级类目的二级类目
    sort: number
    isService?: boolean // 服务大类（固定"服务"：仅后台与团队发布服务商品）
}

// 团队（团队可发布服务类商品）
export interface TeamMember {
    phone: string
    nickName: string
    joinTime: number
}

export interface Team {
    id: string
    name: string
    ownerPhone: string
    ownerName: string
    businessAmount: number // 团队经营金额（元）
    treasury: number // 团队金库（服务订单 90% 分成收入）
    createdAt: number
    members: TeamMember[]
}

// 用户购物车历史（记录商品 id 与数量）
export interface CartItem {
    productId: string
    quantity: number
}

export interface UserCart {
    userId: string
    items: CartItem[]
}

// 管理后台账号
export interface AdminAccount {
    id: string
    username: string
    role: 'admin' | 'staff'
    status: number // 1 启用 / 0 停用
    createdAt: number
}

// 订单（服务器持久化，含支付与物流）
export interface OrderItem {
    id: string
    productId: string
    name: string
    price: number
    count: number
    emoji: string
    colors: string[]
    images: string[]
    attrs?: { name: string; value: string }[]
    attrsText?: string
    service?: boolean
    sourceTeam?: string
}

export interface OrderAddress {
    id?: string
    name?: string
    phone?: string
    region?: string
    detail?: string
    isDefault?: boolean
}

export interface Order {
    id: string
    orderNo: string
    userId: string
    status: 'pending' | 'paid' | 'shipped' | 'done' | 'canceled'
    total: number
    address: OrderAddress
    remark?: string
    payMethod?: string
    payTime?: number
    shipCompany?: string
    shipNo?: string
    shipTime?: number
    finishTime?: number
    cancelTime?: number
    createTime: number
    items: OrderItem[]
}

// 提现申请（服务器绑定企业银行卡号处理）
export interface Withdrawal {
    id: string
    userId: string
    amount: number
    fee: number
    method: string // wechat / alipay / bank
    account: string
    bankCardNo: string // 企业银行卡号（服务器写入）
    status: 'processing' | 'done' | 'failed'
    applyTime: number
    finishTime: number
    remark?: string
}

// 客服会话
export interface SupportTicket {
    id: string
    userId: string
    userPhone: string
    userName: string
    subject: string
    productId: string
    productName: string
    service: boolean
    sourceTeam: string
    assigneeType: 'admin' | 'team'
    assigneePhone: string
    status: 'open' | 'closed'
    lastMessage: string
    lastTime: number
    createdAt: number
}

export interface SupportMessage {
    id: string
    ticketId: string
    senderType: 'user' | 'admin' | 'team'
    senderId: string
    senderName: string
    content: string
    createdAt: number
    read: boolean
}

// 状态文案映射
export const STATUS_TEXT: Record<string, string> = {
    online: '在线',
    offline: '离线',
    alarm: '报警中',
}

export const MSG_TYPE_TEXT: Record<string, string> = {
    alarm: '🚨 报警',
    status: '⚠️ 提醒',
    info: '📄 通知',
}
