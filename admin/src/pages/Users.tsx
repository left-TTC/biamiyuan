import { useEffect, useMemo, useState } from 'react'
import { ShoppingCart, X } from 'lucide-react'
import { api } from '../api'
import type { Product, User, UserCart } from '../types'

function formatTime(ts: number): string {
    const d = new Date(ts)
    const p = (n: number) => String(n).padStart(2, '0')
    return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

export default function Users() {
    const [users, setUsers] = useState<User[]>([])
    const [carts, setCarts] = useState<UserCart[]>([])
    const [products, setProducts] = useState<Product[]>([])
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState('')
    const [viewCart, setViewCart] = useState<UserCart | null>(null) // 正在查看购物车的用户

    useEffect(() => {
        Promise.all([api.users(), api.userCarts(), api.products()])
            .then(([u, c, p]) => {
                setUsers(u)
                setCarts(c || [])
                setProducts(p || [])
                setLoading(false)
            })
            .catch((err) => {
                setError(err instanceof Error ? err.message : '加载失败')
                setLoading(false)
            })
    }, [])

    const productMap = useMemo(() => {
        const m: Record<string, Product> = {}
        products.forEach((p) => {
            m[p.id] = p
        })
        return m
    }, [products])

    const cartOf = (userId: string): UserCart | undefined => carts.find((c) => c.userId === userId)

    if (loading) return <div className="loading">加载中…</div>
    if (error) return <div className="error-box">{error}</div>

    return (
        <div className="card">
            <div className="page-title">用户管理（{users.length}）</div>
            {users.length === 0 ? (
                <div className="empty">暂无用户（服务器为内存存储，重启后数据清空）</div>
            ) : (
                <div className="table-wrap">
                    <table>
                        <thead>
                            <tr>
                                <th>手机号</th>
                                <th>昵称</th>
                                <th>角色</th>
                                <th>邀请码</th>
                                <th>余额</th>
                                <th>累计佣金</th>
                                <th>报警通知</th>
                                <th>openid</th>
                                <th>购物车</th>
                                <th>创建时间</th>
                            </tr>
                        </thead>
                        <tbody>
                            {users.map((u) => (
                                <tr key={u.id}>
                                    <td>{u.phone}</td>
                                    <td>
                                        {u.avatarUrl ? <img className="user-avatar" src={u.avatarUrl} alt="" /> : null}
                                        {u.nickName}
                                    </td>
                                    <td>
                                        <span className="badge badge-info">{u.role}</span>
                                    </td>
                                    <td>{u.promoterCode}</td>
                                    <td>¥{u.balance.toFixed(2)}</td>
                                    <td>¥{u.totalCommission.toFixed(2)}</td>
                                    <td>{u.notifyEnabled ? '已开启' : '关闭'}</td>
                                    <td>{u.openid ? u.openid.slice(0, 12) + '…' : '-'}</td>
                                    <td>
                                        <button
                                            className="btn btn-outline btn-xs"
                                            onClick={() => setViewCart(cartOf(u.id) || { userId: u.id, items: [] })}
                                        >
                                            <ShoppingCart size={12} />
                                            {cartOf(u.id)?.items.length || 0} 件
                                        </button>
                                    </td>
                                    <td>{formatTime(u.createdAt)}</td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}
            {/* 购物车历史弹窗 */}
            {viewCart && (
                <div className="modal-mask">
                    <div className="modal">
                        <div className="modal-head">
                            <span className="modal-title">
                                购物车历史 · {users.find((u) => u.id === viewCart.userId)?.phone || viewCart.userId}
                            </span>
                            <button className="modal-close" onClick={() => setViewCart(null)}>
                                <X size={18} />
                            </button>
                        </div>
                        <div className="modal-body">
                            {viewCart.items.length === 0 ? (
                                <div className="empty">该用户购物车为空</div>
                            ) : (
                                <div className="table-wrap">
                                    <table>
                                        <thead>
                                            <tr>
                                                <th>商品 ID</th>
                                                <th>商品名称</th>
                                                <th>数量</th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            {viewCart.items.map((it) => {
                                                const p = productMap[it.productId]
                                                return (
                                                    <tr key={it.productId}>
                                                        <td>{it.productId}</td>
                                                        <td>{p ? p.name : '（商品已下架）'}</td>
                                                        <td>×{it.quantity}</td>
                                                    </tr>
                                                )
                                            })}
                                        </tbody>
                                    </table>
                                </div>
                            )}
                            <p className="img-tip">购物车历史存于服务器 SQLite，记录商品 id 与数量</p>
                        </div>
                        <div className="modal-foot">
                            <button className="btn btn-outline" onClick={() => setViewCart(null)}>
                                关闭
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    )
}
