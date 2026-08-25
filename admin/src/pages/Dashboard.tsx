import { useEffect, useState } from 'react'
import {
    Users,
    Radio,
    Siren,
    MessageSquare,
    ShoppingCart,
    FolderOpen,
    Database,
    CheckCircle2,
    XCircle,
} from 'lucide-react'
import { api } from '../api'
import type { DBStatus, Stats } from '../types'

interface StatItem {
    label: string
    value: number
    icon: typeof Users
    cls: string
    color: string
}

function formatTime(ts: number): string {
    const d = new Date(ts)
    const p = (n: number) => String(n).padStart(2, '0')
    return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

const FIELD_TEXT: Record<string, string> = {
    price: '价格',
    originalPrice: '原价',
    sales: '销量',
}

export default function Dashboard() {
    const [stats, setStats] = useState<Stats | null>(null)
    const [db, setDb] = useState<DBStatus | null>(null)
    const [error, setError] = useState('')

    useEffect(() => {
        Promise.all([api.stats(), api.dbStatus()])
            .then(([s, d]) => {
                setStats(s)
                setDb(d)
            })
            .catch((err) => setError(err instanceof Error ? err.message : '加载失败'))
    }, [])

    if (error) return <div className="error-box">{error}</div>
    if (!stats || !db) return <div className="loading">加载中…</div>

    const items: StatItem[] = [
        { label: '用户总数', value: stats.users, icon: Users, cls: 'blue', color: '#1e6fff' },
        { label: '设备总数', value: stats.devices, icon: Radio, cls: 'green', color: '#10b981' },
        { label: '报警中设备', value: stats.alarms, icon: Siren, cls: 'red', color: '#f43f5e' },
        { label: '消息总数', value: stats.messages, icon: MessageSquare, cls: 'orange', color: '#f59e0b' },
        { label: '商品总数', value: stats.products, icon: ShoppingCart, cls: 'purple', color: '#8b5cf6' },
        { label: '商品分类', value: stats.categories, icon: FolderOpen, cls: 'blue', color: '#1e6fff' },
    ]

    return (
        <div>
            <div className="page-title">数据概览</div>
            <div className="stat-grid">
                {items.map((it) => (
                    <div key={it.label} className="stat-card">
                        <div className={`stat-icon ${it.cls}`}>
                            <it.icon size={24} color={it.color} strokeWidth={1.8} />
                        </div>
                        <div>
                            <div className="stat-num">{it.value}</div>
                            <div className="stat-label">{it.label}</div>
                        </div>
                    </div>
                ))}
            </div>

            {/* 数据库连接状态（验证） */}
            <div className="card db-card">
                <div className="page-title">数据库连接</div>
                <div className="db-head">
                    <div className={`db-indicator ${db.connected ? 'ok' : 'fail'}`}>
                        <Database size={18} color={db.connected ? '#10b981' : '#f43f5e'} />
                        <span>{db.connected ? '已连接' : '未连接'}</span>
                        {db.connected ? (
                            <CheckCircle2 size={16} color="#10b981" />
                        ) : (
                            <XCircle size={16} color="#f43f5e" />
                        )}
                    </div>
                    <div className="db-meta">
                        <span className="db-meta-item">驱动：{db.driver || '-'}</span>
                        <span className="db-meta-item">路径：{db.path || '-'}</span>
                        <span className="db-meta-item">数据表：{db.tables.join(', ') || '-'}</span>
                        <span className="db-meta-item">商品修改审计：{db.productEdits} 条</span>
                    </div>
                </div>

                <div className="db-edits">
                    <div className="db-edits-title">最近商品修改记录（审计）</div>
                    {db.recentEdits.length > 0 ? (
                        <div className="db-edit-list">
                            {db.recentEdits.map((e) => (
                                <div key={e.id} className="db-edit-item">
                                    <span className="db-edit-field">{FIELD_TEXT[e.field] || e.field}</span>
                                    <span className="db-edit-old">{e.oldValue}</span>
                                    <span className="db-edit-arrow">→</span>
                                    <span className="db-edit-new">{e.newValue}</span>
                                    <span className="db-edit-time">{formatTime(e.createdAt)}</span>
                                </div>
                            ))}
                        </div>
                    ) : (
                        <div className="empty">暂无修改记录（在商品管理页编辑商品价格/销量后生成）</div>
                    )}
                </div>
            </div>
        </div>
    )
}
