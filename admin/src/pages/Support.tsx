import { useCallback, useEffect, useState } from 'react'
import { Headphones, Send, X } from 'lucide-react'
import { api } from '../api'
import type { SupportMessage, SupportTicket } from '../types'

const TABS = [
    { key: 'all', label: '全部' },
    { key: 'open', label: '待处理' },
    { key: 'closed', label: '已关闭' },
]

export default function Support() {
    const [tickets, setTickets] = useState<SupportTicket[]>([])
    const [tab, setTab] = useState('all')
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState('')
    const [busy, setBusy] = useState('')
    // 会话详情
    const [current, setCurrent] = useState<SupportTicket | null>(null)
    const [messages, setMessages] = useState<SupportMessage[]>([])
    const [reply, setReply] = useState('')

    const load = useCallback(() => {
        api.supportTickets(tab)
            .then((data) => {
                setTickets(data)
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

    const open = async (id: string) => {
        setBusy(id)
        try {
            const data = await api.supportTicketDetail(id)
            setCurrent(data.ticket)
            setMessages(data.messages)
            setReply('')
        } catch (err) {
            alert(err instanceof Error ? err.message : '加载失败')
        } finally {
            setBusy('')
        }
    }

    const sendReply = async () => {
        if (!current || !reply.trim()) return
        setBusy(current.id + '_r')
        try {
            await api.replySupport(current.id, reply.trim())
            setReply('')
            const data = await api.supportTicketDetail(current.id)
            setMessages(data.messages)
        } catch (err) {
            alert(err instanceof Error ? err.message : '发送失败')
        } finally {
            setBusy('')
        }
    }

    const close = async () => {
        if (!current) return
        if (!window.confirm('确定关闭该会话？')) return
        setBusy(current.id + '_c')
        try {
            await api.closeSupport(current.id)
            setCurrent(null)
            setMessages([])
            load()
        } catch (err) {
            alert(err instanceof Error ? err.message : '关闭失败')
        } finally {
            setBusy('')
        }
    }

    const assigneeText = (t: SupportTicket) =>
        t.service && t.sourceTeam
            ? `团队服务 · ${t.sourceTeam}`
            : t.assigneeType === 'team'
              ? `团队客服 ${t.assigneePhone}`
              : '后台客服'

    return (
        <div className="page">
            <div className="page-head">
                <div>
                    <div className="page-title">客服工作台</div>
                    <div className="page-sub">普通商品/官方服务 → 后台客服回复；团队服务 → 团队指定客服成员在小程序端回复</div>
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
            ) : tickets.length === 0 ? (
                <div className="empty">
                    <Headphones size={48} color="#c0c6cf" />
                    <div className="empty-text">暂无客服会话</div>
                </div>
            ) : (
                <div className="table-wrap">
                    <table>
                        <thead>
                            <tr>
                                <th>主题</th>
                                <th>用户</th>
                                <th>关联商品</th>
                                <th>分配</th>
                                <th>最后消息</th>
                                <th>状态</th>
                                <th>时间</th>
                                <th>操作</th>
                            </tr>
                        </thead>
                        <tbody>
                            {tickets.map((t) => (
                                <tr key={t.id}>
                                    <td>{t.subject}</td>
                                    <td>{t.userName || t.userPhone}</td>
                                    <td>
                                        {t.productName ? (
                                            <>
                                                {t.productName}
                                                {t.service ? <span className="badge badge-info">服务</span> : null}
                                            </>
                                        ) : (
                                            '—'
                                        )}
                                    </td>
                                    <td className="muted">{assigneeText(t)}</td>
                                    <td className="muted" style={{ maxWidth: 200 }}>
                                        {t.lastMessage}
                                    </td>
                                    <td>
                                        <span className={`badge ${t.status === 'open' ? 'badge-warning' : 'badge'}`}>
                                            {t.status === 'open' ? '待处理' : '已关闭'}
                                        </span>
                                    </td>
                                    <td>{new Date(t.lastTime || t.createdAt).toLocaleString('zh-CN')}</td>
                                    <td className="inline-edit">
                                        <button
                                            className="btn btn-outline btn-sm"
                                            disabled={busy === t.id}
                                            onClick={() => open(t.id)}
                                        >
                                            回复
                                        </button>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}


            {current && (
                <div className="modal-mask">
                    <div className="modal support-modal">
                        <div className="modal-head">
                            <span className="modal-title">
                                {current.subject}
                                <span className="muted"> · {current.userName || current.userPhone}</span>
                            </span>
                            <button className="modal-close" onClick={() => setCurrent(null)}>
                                <X size={18} />
                            </button>
                        </div>
                        <div className="modal-body">
                            <div className="support-info muted">
                                {assigneeText(current)} · {current.status === 'open' ? '待处理' : '已关闭'}
                            </div>
                            <div className="chat-box">
                                {messages.map((m) => (
                                    <div key={m.id} className={`chat-msg ${m.senderType === 'user' ? 'left' : 'right'}`}>
                                        <div className="chat-name">
                                            {m.senderType === 'user' ? m.senderName || '用户' : m.senderName || '客服'}
                                        </div>
                                        <div className="chat-bubble">{m.content}</div>
                                    </div>
                                ))}
                            </div>
                            <div className="chat-input">
                                <input
                                    className="form-input"
                                    value={reply}
                                    onChange={(e) => setReply(e.target.value)}
                                    placeholder="输入回复内容…"
                                    onKeyDown={(e) => {
                                        if (e.key === 'Enter' && !e.shiftKey) sendReply()
                                    }}
                                />
                                <button
                                    className="btn"
                                    disabled={busy === current.id + '_r' || !reply.trim()}
                                    onClick={sendReply}
                                >
                                    <Send size={14} /> 发送
                                </button>
                            </div>
                        </div>
                        <div className="modal-foot">
                            {current.status === 'open' && (
                                <button
                                    className="btn btn-danger-outline"
                                    disabled={busy === current.id + '_c'}
                                    onClick={close}
                                >
                                    关闭会话
                                </button>
                            )}
                        </div>
                    </div>
                </div>
            )}
        </div>
    )
}

