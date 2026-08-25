import { useCallback, useEffect, useState } from 'react'
import { Package, Truck } from 'lucide-react'
import { api } from '../api'
import type { Order } from '../types'

const STATUS_TEXT: Record<string, string> = {
    pending: '待付款',
    paid: '待发货',
    shipped: '待收货',
    done: '已完成',
    canceled: '已取消',
    refunded: '已退款',
}

const TABS = [
    { key: 'all', label: '全部' },
    { key: 'paid', label: '待发货' },
    { key: 'shipped', label: '待收货' },
    { key: 'done', label: '已完成' },
    { key: 'pending', label: '待付款' },
    { key: 'canceled', label: '已取消' },
    { key: 'refunded', label: '已退款' },
]

export default function Orders() {
    const [orders, setOrders] = useState<Order[]>([])
    const [tab, setTab] = useState('all')
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState('')
    const [busy, setBusy] = useState('')
    // 发货弹窗
    const [shipOrder, setShipOrder] = useState<Order | null>(null)
    const [shipCompany, setShipCompany] = useState('')
    const [shipNo, setShipNo] = useState('')

    const load = useCallback(() => {
        setLoading(true)
        api.orders(tab)
            .then((data) => {
                setOrders(data)
                setLoading(false)
            })
            .catch((err) => {
                setError(err instanceof Error ? err.message : '加载失败')
                setLoading(false)
            })
    }, [tab])

    useEffect(() => {
        load()
    }, [load])

    const doShip = async () => {
        if (!shipOrder) return
        if (!shipCompany.trim() || !shipNo.trim()) {
            alert('请填写物流公司与物流单号')
            return
        }
        setBusy(shipOrder.id)
        try {
            await api.shipOrder(shipOrder.id, shipCompany.trim(), shipNo.trim())
            setShipOrder(null)
            setShipCompany('')
            setShipNo('')
            load()
        } catch (err) {
            alert(err instanceof Error ? err.message : '发货失败')
        } finally {
            setBusy('')
        }
    }

    const fmt = (n?: number) => (n || 0).toLocaleString('zh-CN', { maximumFractionDigits: 2 })
    const statusClass = (s: string) =>
        s === 'done'
            ? 'badge badge-success'
            : s === 'canceled' || s === 'refunded'
              ? 'badge badge-danger'
              : s === 'shipped'
                ? 'badge badge-warning'
                : 'badge badge-info'

    return (
        <div className="page">
            <div className="page-head">
                <div>
                    <div className="page-title">订单管理</div>
                    <div className="page-sub">购买 · 支付 · 物流绑定（订单价格以服务器商品表为准）</div>
                </div>
            </div>

            <div className="tab-bar">
                {TABS.map((t) => (
                    <button
                        key={t.key}
                        className={`tab-item ${tab === t.key ? 'active' : ''}`}
                        onClick={() => setTab(t.key)}
                    >
                        {t.label}
                    </button>
                ))}
            </div>

            {loading ? (
                <div className="loading">加载中…</div>
            ) : error ? (
                <div className="error-box">{error}</div>
            ) : orders.length === 0 ? (
                <div className="empty">
                    <Package size={48} color="#c0c6cf" />
                    <div className="empty-text">暂无订单</div>
                </div>
            ) : (
                <div className="table-wrap">
                    <table>
                        <thead>
                            <tr>
                                <th>订单号</th>
                                <th>商品</th>
                                <th>金额</th>
                                <th>收货人</th>
                                <th>物流</th>
                                <th>状态</th>
                                <th>下单时间</th>
                                <th>操作</th>
                            </tr>
                        </thead>
                        <tbody>
                            {orders.map((o) => (
                                <tr key={o.id}>
                                    <td className="mono">{o.orderNo}</td>
                                    <td>
                                        {(o.items || []).map((it, i) => (
                                            <div key={i}>
                                                {it.emoji} {it.name} × {it.count}
                                                {it.attrsText ? <span className="muted">（{it.attrsText}）</span> : null}
                                                {it.service ? <span className="badge badge-info">服务</span> : null}
                                            </div>
                                        ))}
                                    </td>
                                    <td>¥{fmt(o.total)}</td>
                                    <td>
                                        {o.address ? (
                                            <>
                                                {o.address.name} {o.address.phone}
                                                <br />
                                                <span className="muted">
                                                    {o.address.region} {o.address.detail}
                                                </span>
                                            </>
                                        ) : (
                                            '—'
                                        )}
                                    </td>
                                    <td>
                                        {o.shipCompany && o.shipNo ? (
                                            <>
                                                {o.shipCompany}
                                                <br />
                                                <span className="mono">{o.shipNo}</span>
                                            </>
                                        ) : (
                                            '未发货'
                                        )}
                                    </td>
                                    <td>
                                        <span className={statusClass(o.status)}>
                                            {STATUS_TEXT[o.status] || o.status}
                                        </span>
                                    </td>
                                    <td>{new Date(o.createTime).toLocaleString('zh-CN')}</td>
                                    <td className="inline-edit">
                                        {o.status === 'paid' && (
                                            <button
                                                className="btn btn-outline btn-sm"
                                                disabled={busy === o.id}
                                                onClick={() => {
                                                    setShipOrder(o)
                                                    setShipCompany('')
                                                    setShipNo('')
                                                }}
                                            >
                                                <Truck size={12} /> 发货
                                            </button>
                                        )}
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}


            {shipOrder && (
                <div className="modal-mask">
                    <div className="modal">
                        <div className="modal-head">
                            <span className="modal-title">订单发货 · {shipOrder.orderNo}</span>
                            <button className="modal-close" onClick={() => setShipOrder(null)}>
                                ×
                            </button>
                        </div>
                        <div className="modal-body">
                            <div className="form-group">
                                <label>物流公司 *</label>
                                <input
                                    className="form-input"
                                    value={shipCompany}
                                    onChange={(e) => setShipCompany(e.target.value)}
                                    placeholder="如：顺丰速运"
                                />
                            </div>
                            <div className="form-group">
                                <label>物流单号 *</label>
                                <input
                                    className="form-input"
                                    value={shipNo}
                                    onChange={(e) => setShipNo(e.target.value)}
                                    placeholder="如：SF1234567890"
                                />
                            </div>
                        </div>
                        <div className="modal-foot">
                            <button className="btn btn-outline" onClick={() => setShipOrder(null)}>
                                取消
                            </button>
                            <button className="btn" disabled={busy === shipOrder.id} onClick={doShip}>
                                {busy === shipOrder.id ? '发货中…' : '确认发货'}
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    )
}

