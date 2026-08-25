import { useCallback, useEffect, useState } from 'react'
import { Trash2 } from 'lucide-react'
import { api } from '../api'
import { MSG_TYPE_TEXT, type Message } from '../types'

function formatTime(ts: number): string {
    const d = new Date(ts)
    const p = (n: number) => String(n).padStart(2, '0')
    return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

export default function Messages() {
    const [messages, setMessages] = useState<Message[]>([])
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState('')
    const [busy, setBusy] = useState('')

    const load = useCallback(() => {
        api.messages()
            .then((data) => {
                setMessages(data)
                setLoading(false)
            })
            .catch((err) => {
                setError(err instanceof Error ? err.message : '加载失败')
                setLoading(false)
            })
    }, [])

    useEffect(() => {
        load()
    }, [load])

    const remove = async (id: string) => {
        if (!window.confirm('确定删除该消息？')) return
        setBusy(id)
        try {
            await api.removeMessage(id)
            load()
        } catch (err) {
            alert(err instanceof Error ? err.message : '操作失败')
        } finally {
            setBusy('')
        }
    }

    if (loading) return <div className="loading">加载中…</div>
    if (error) return <div className="error-box">{error}</div>

    return (
        <div className="card">
            <div className="page-title">消息管理（{messages.length}）</div>
            <div className="table-wrap">
                <table>
                    <thead>
                        <tr>
                            <th>类型</th>
                            <th>标题</th>
                            <th>内容</th>
                            <th>设备ID</th>
                            <th>状态</th>
                            <th>时间</th>
                            <th>操作</th>
                        </tr>
                    </thead>
                    <tbody>
                        {messages.map((m) => (
                            <tr key={m.id}>
                                <td>
                                    <span className={`badge ${m.type === 'alarm' ? 'badge-alarm' : m.type === 'status' ? 'badge-warn' : 'badge-info'}`}>
                                        {MSG_TYPE_TEXT[m.type] || m.type}
                                    </span>
                                </td>
                                <td>{m.title}</td>
                                <td style={{ maxWidth: 320, whiteSpace: 'normal' }}>{m.content}</td>
                                <td>{m.deviceId.slice(0, 12)}…</td>
                                <td>{m.read ? '已读' : '未读'}</td>
                                <td>{formatTime(m.time)}</td>
                                <td>
                                    <button
                                        className="btn btn-danger-outline btn-sm"
                                        disabled={busy === m.id}
                                        onClick={() => remove(m.id)}
                                    >
                                        <Trash2 size={14} />
                                        删除
                                    </button>
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </table>
                {messages.length === 0 && <div className="empty">暂无消息</div>}
            </div>
        </div>
    )
}
