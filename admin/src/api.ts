// API 请求封装（对接 Go 服务器管理接口）
import type {
    AdminAccount,
    Category,
    DBStatus,
    Device,
    Message,
    Order,
    Product,
    Stats,
    SupportMessage,
    SupportTicket,
    Team,
    User,
    UserCart,
    Withdrawal,
} from './types'

// 开发时通过 vite 代理 /api -> localhost:8080；构建后可用 VITE_API_BASE 指定完整地址
const BASE = import.meta.env.VITE_API_BASE || ''

const TOKEN_KEY = 'admin_token'
const ROLE_KEY = 'admin_role'
const USERNAME_KEY = 'admin_username'

export function getToken(): string | null {
    return localStorage.getItem(TOKEN_KEY)
}

function setToken(token: string) {
    localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken() {
    localStorage.removeItem(TOKEN_KEY)
}

export function getRole(): string {
    return localStorage.getItem(ROLE_KEY) || 'staff'
}

export function getUsername(): string {
    return localStorage.getItem(USERNAME_KEY) || ''
}

// 登出：清除登录态
export function clearSession() {
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(ROLE_KEY)
    localStorage.removeItem(USERNAME_KEY)
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
    const token = getToken()
    const res = await fetch(BASE + path, {
        ...options,
        headers: {
            'Content-Type': 'application/json',
            ...(token ? { Authorization: `Bearer ${token}` } : {}),
            ...(options.headers || {}),
        },
    })
    const body = await res.json().catch(() => null)
    if (!res.ok || !body || body.code !== 0) {
        throw new Error((body && body.msg) || `请求失败 (${res.status})`)
    }
    return body.data as T
}

export const api = {
    // 管理端账号登录（admin / staff），成功保存 token 与角色
    adminLogin: async (username: string, password: string): Promise<{ role: string; username: string }> => {
        const data = await request<{ token: string; role: string; username: string }>('/api/v1/admin/login', {
            method: 'POST',
            body: JSON.stringify({ username, password }),
        })
        setToken(data.token)
        localStorage.setItem(ROLE_KEY, data.role)
        localStorage.setItem(USERNAME_KEY, data.username)
        return data
    },

    stats: () => request<Stats>('/api/v1/admin/stats'),
    dbStatus: () => request<DBStatus>('/api/v1/admin/db-status'),
    users: () => request<User[]>('/api/v1/admin/users'),
    userCarts: () => request<UserCart[]>('/api/v1/admin/carts'),
    devices: () => request<Device[]>('/api/v1/admin/devices'),
    messages: () => request<Message[]>('/api/v1/admin/messages'),
    teams: () => request<Team[]>('/api/v1/admin/teams'),
    removeTeam: (id: string) =>
        request<{ id: string }>(`/api/v1/admin/teams/${id}`, { method: 'DELETE' }),
    products: () => request<Product[]>('/api/v1/admin/products'),

    updateProduct: (id: string, data: Partial<Product>) =>
        request<Product>(`/api/v1/admin/products/${id}`, {
            method: 'PUT',
            body: JSON.stringify(data),
        }),

    deviceAlarm: (id: string) =>
        request<{ device: Device; message: Message }>(`/api/v1/admin/devices/${id}/alarm`, {
            method: 'POST',
            body: '{}',
        }),

    removeDevice: (id: string) =>
        request<{ id: string }>(`/api/v1/admin/devices/${id}`, { method: 'DELETE' }),

    removeMessage: (id: string) =>
        request<{ id: string }>(`/api/v1/admin/messages/${id}`, { method: 'DELETE' }),

    // 类目管理
    categories: () => request<Category[]>('/api/v1/admin/categories'),
    createCategory: (data: Partial<Category>) =>
        request<Category>('/api/v1/admin/categories', {
            method: 'POST',
            body: JSON.stringify(data),
        }),
    updateCategory: (id: string, data: Partial<Category>) =>
        request<Category>(`/api/v1/admin/categories/${id}`, {
            method: 'PUT',
            body: JSON.stringify(data),
        }),
    deleteCategory: (id: string) =>
        request<{ id: string }>(`/api/v1/admin/categories/${id}`, { method: 'DELETE' }),

    // 商品新增 / 删除
    createProduct: (data: Partial<Product>) =>
        request<Product>('/api/v1/admin/products', {
            method: 'POST',
            body: JSON.stringify(data),
        }),
    deleteProduct: (id: string) =>
        request<{ id: string }>(`/api/v1/admin/products/${id}`, { method: 'DELETE' }),

    // Excel 导出（前端用 SheetJS 生成 xlsx，浏览器下载）
    exportProducts: async (products: Product[]): Promise<void> => {
        const XLSX = await import('xlsx')
        const rows = products.map((p) => ({
            id: p.id,
            名称: p.name,
            描述: p.desc,
            价格: p.price,
            原价: p.originalPrice,
            图标: p.emoji,
            '颜色(JSON)': JSON.stringify(p.colors),
            '图片(JSON)': JSON.stringify(p.images || []),
            销量: p.sales,
            类目ID: p.category,
            '标签(JSON)': JSON.stringify(p.tags),
            '详情(JSON)': JSON.stringify(p.detail),
        }))
        const ws = XLSX.utils.json_to_sheet(rows)
        ws['!cols'] = [
            { wch: 10 }, { wch: 24 }, { wch: 32 }, { wch: 8 }, { wch: 8 },
            { wch: 6 }, { wch: 30 }, { wch: 30 }, { wch: 8 }, { wch: 10 }, { wch: 28 }, { wch: 28 },
        ]
        const wb = XLSX.utils.book_new()
        XLSX.utils.book_append_sheet(wb, ws, '商品')
        XLSX.writeFile(wb, `products_${new Date().toISOString().slice(0, 10)}.xlsx`)
    },

    // Excel 导入（前端解析 xlsx 为商品数组）
    parseProductsFile: async (file: File): Promise<Product[]> => {
        const XLSX = await import('xlsx')
        const buffer = await file.arrayBuffer()
        const wb = XLSX.read(buffer)
        const ws = wb.Sheets[wb.SheetNames[0]]
        const rows = XLSX.utils.sheet_to_json<Record<string, unknown>>(ws)
        const parseArr = (v: unknown): string[] => {
            if (Array.isArray(v)) return v as string[]
            if (typeof v === 'string') {
                try {
                    const parsed = JSON.parse(v)
                    return Array.isArray(parsed) ? (parsed as string[]) : []
                } catch {
                    return []
                }
            }
            return []
        }
        return rows.map((r) => ({
            id: String(r['id'] ?? ''),
            name: String(r['名称'] ?? ''),
            desc: String(r['描述'] ?? ''),
            price: Number(r['价格']) || 0,
            originalPrice: Number(r['原价']) || Number(r['价格']) || 0,
            emoji: String(r['图标'] ?? '📦'),
            colors: parseArr(r['颜色(JSON)']),
            images: parseArr(r['图片(JSON)']),
            sales: Number(r['销量']) || 0,
            category: String(r['类目ID'] ?? ''),
            tags: parseArr(r['标签(JSON)']),
            detail: parseArr(r['详情(JSON)']),
        }))
    },

    // 批量导入：前端解析后的商品数组提交服务器（服务器校验 + 审计）
    importProducts: async (products: Product[]): Promise<{ total: number; imported: number; updated: number; failed: number }> => {
        return request('/api/v1/admin/products/batch', {
            method: 'POST',
            body: JSON.stringify({ products }),
        })
    },

    // 上传商品图片（返回相对 URL，如 /uploads/img_xxx.jpg）
    uploadProductImage: async (file: File): Promise<{ url: string; name: string }> => {
        const token = getToken()
        const form = new FormData()
        form.append('file', file)
        const res = await fetch(BASE + '/api/v1/admin/products/upload', {
            method: 'POST',
            headers: token ? { Authorization: `Bearer ${token}` } : {},
            body: form,
        })
        const body = await res.json().catch(() => null)
        if (!res.ok || !body || body.code !== 0) {
            throw new Error((body && body.msg) || `上传失败 (${res.status})`)
        }
        return body.data
    },

    // 账号管理（仅 admin 可调用，服务端强制校验）
    accounts: () => request<AdminAccount[]>('/api/v1/admin/accounts'),
    createAccount: (data: { username: string; password: string; role?: string }) =>
        request<AdminAccount>('/api/v1/admin/accounts', {
            method: 'POST',
            body: JSON.stringify(data),
        }),
    deleteAccount: (id: string) =>
        request<{ id: string }>(`/api/v1/admin/accounts/${id}`, { method: 'DELETE' }),
    updateAccountRole: (id: string, role: string) =>
        request<{ id: string; role: string }>(`/api/v1/admin/accounts/${id}/role`, {
            method: 'PUT',
            body: JSON.stringify({ role }),
        }),
    updateAccountStatus: (id: string, status: number) =>
        request<{ id: string; status: string }>(`/api/v1/admin/accounts/${id}/status`, {
            method: 'PUT',
            body: JSON.stringify({ status }),
        }),

    // 订单管理（支付 + 物流绑定）
    orders: (status = 'all') => request<Order[]>(`/api/v1/admin/orders?status=${status}`),
    shipOrder: (id: string, company: string, shipNo: string) =>
        request<Order>(`/api/v1/admin/orders/${id}/ship`, {
            method: 'POST',
            body: JSON.stringify({ company, shipNo }),
        }),

    // 提现审核（企业银行卡号由服务器绑定）
    withdrawals: () => request<Withdrawal[]>('/api/v1/admin/withdrawals'),
    completeWithdrawal: (id: string) =>
        request<Withdrawal>(`/api/v1/admin/withdrawals/${id}/complete`, {
            method: 'POST',
            body: '{}',
        }),
    failWithdrawal: (id: string) =>
        request<Withdrawal>(`/api/v1/admin/withdrawals/${id}/fail`, {
            method: 'POST',
            body: '{}',
        }),

    // 客服工作台（后台客服；团队服务由团队客服成员在小程序端处理）
    supportTickets: (status = 'all') => request<SupportTicket[]>(`/api/v1/admin/support/tickets?status=${status}`),
    supportTicketDetail: (id: string) =>
        request<{ ticket: SupportTicket; messages: SupportMessage[] }>(`/api/v1/admin/support/tickets/${id}`),
    replySupport: (id: string, content: string) =>
        request<SupportMessage>(`/api/v1/admin/support/tickets/${id}/reply`, {
            method: 'POST',
            body: JSON.stringify({ content }),
        }),
    closeSupport: (id: string) =>
        request<SupportTicket>(`/api/v1/admin/support/tickets/${id}/close`, {
            method: 'POST',
            body: '{}',
        }),
}
