import { useCallback, useEffect, useState } from 'react'
import { Plus, Trash2, X, Power, UserCog } from 'lucide-react'
import { api, getUsername } from '../api'
import type { AdminAccount } from '../types'

function formatTime(ts: number): string {
    const d = new Date(ts)
    const p = (n: number) => String(n).padStart(2, '0')
    return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

const emptyForm = { username: '', password: '', role: 'staff' }

export default function Accounts() {
    const [list, setList] = useState<AdminAccount[]>([])
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState('')
    const [showForm, setShowForm] = useState(false)
    const [form, setForm] = useState(emptyForm)
    const [busy, setBusy] = useState('')
    const myUsername = getUsername()

    const load = useCallback(() => {
        api.accounts()
            .then((data) => {
                setList(data)
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

    const create = async () => {
        if (form.username.trim().length < 3) {
            alert('用户名至少 3 个字符')
            return
        }
        if (form.password.length < 6) {
            alert('密码至少 6 位')
            return
        }
        setBusy('new')
        try {
            await api.createAccount({
                username: form.username.trim(),
                password: form.password,
                role: form.role,
            })
            setShowForm(false)
            setForm(emptyForm)
            load()
        } catch (err) {
            alert(err instanceof Error ? err.message : '创建失败')
        } finally {
            setBusy('')
        }
    }

    const toggleRole = async (acc: AdminAccount) => {
        const next = acc.role === 'admin' ? 'staff' : 'admin'
        if (!window.confirm(`确定将「${acc.username}」的权限改为「${next === 'admin' ? '超级管理员' : '员工'}」？`)) return
        setBusy(acc.id)
        try {
            await api.updateAccountRole(acc.id, next)
            load()
        } catch (err) {
            alert(err instanceof Error ? err.message : '操作失败')
        } finally {
            setBusy('')
        }
    }

    const toggleStatus = async (acc: AdminAccount) => {
        const next = acc.status === 1 ? 0 : 1
        setBusy(acc.id)
        try {
            await api.updateAccountStatus(acc.id, next)
            load()
        } catch (err) {
            alert(err instanceof Error ? err.message : '操作失败')
        } finally {
            setBusy('')
        }
    }

    const remove = async (acc: AdminAccount) => {
        if (!window.confirm(`确定销毁账号「${acc.username}」？该账号将无法再登录`)) return
        setBusy(acc.id)
        try {
            await api.deleteAccount(acc.id)
            load()
        } catch (err) {
            alert(err instanceof Error ? err.message : '删除失败')
        } finally {
            setBusy('')
        }
    }

    if (loading) return <div className="loading">加载中…</div>
    if (error) return <div className="error-box">{error}</div>

    return (
        <div className="card">
            <div className="page-title">
                账号管理（{list.length}）
                <button className="btn btn-sm" style={{ marginLeft: 'auto' }} onClick={() => setShowForm(true)}>
                    <Plus size={14} />
                    创建员工
                </button>
            </div>
            <p className="img-tip" style={{ marginBottom: 12 }}>
                员工账号只能管理业务数据，无法访问账号管理与权限修改（服务端强制校验）。
            </p>
            <div className="table-wrap">
                <table>
                    <thead>
                        <tr>
                            <th>用户名</th>
                            <th>角色</th>
                            <th>状态</th>
                            <th>创建时间</th>
                            <th>操作</th>
                        </tr>
                    </thead>
                    <tbody>
                        {list.map((a) => {
                            const isMe = a.username === myUsername
                            return (
                                <tr key={a.id}>
                                    <td style={{ fontWeight: 600 }}>
                                        {a.username}
                                        {isMe && <span className="badge badge-info" style={{ marginLeft: 8 }}>当前</span>}
                                    </td>
                                    <td>
                                        <span className={`badge ${a.role === 'admin' ? 'badge-info' : 'badge-warn'}`}>
                                            {a.role === 'admin' ? '👑 超级管理员' : '👤 员工'}
                                        </span>
                                    </td>
                                    <td>
                                        <span className={`badge ${a.status === 1 ? 'badge-online' : 'badge-offline'}`}>
                                            {a.status === 1 ? '启用' : '停用'}
                                        </span>
                                    </td>
                                    <td>{formatTime(a.createdAt)}</td>
                                    <td className="inline-edit">
                                        <button
                                            className="btn btn-outline btn-sm"
                                            disabled={busy === a.id || isMe}
                                            onClick={() => toggleRole(a)}
                                        >
                                            <UserCog size={14} />
                                            {a.role === 'admin' ? '降为员工' : '升为管理员'}
                                        </button>
                                        <button
                                            className="btn btn-warning btn-sm"
                                            disabled={busy === a.id || isMe}
                                            onClick={() => toggleStatus(a)}
                                        >
                                            <Power size={14} />
                                            {a.status === 1 ? '停用' : '启用'}
                                        </button>
                                        <button
                                            className="btn btn-danger-outline btn-sm"
                                            disabled={busy === a.id || isMe || a.role === 'admin'}
                                            onClick={() => remove(a)}
                                        >
                                            <Trash2 size={14} />
                                            销毁
                                        </button>
                                    </td>
                                </tr>
                            )
                        })}
                    </tbody>
                </table>
                {list.length === 0 && <div className="empty">暂无账号</div>}
            </div>

            {/* 创建员工弹窗 */}
            {showForm && (
                <div className="modal-mask">
                    <div className="modal">
                        <div className="modal-head">
                            <span className="modal-title">创建员工账号</span>
                            <button className="modal-close" onClick={() => setShowForm(false)}>
                                <X size={18} />
                            </button>
                        </div>
                        <div className="modal-body">
                            <div className="form-group">
                                <label>用户名 *（至少 3 个字符）</label>
                                <input
                                    className="form-input"
                                    value={form.username}
                                    onChange={(e) => setForm({ ...form, username: e.target.value })}
                                    placeholder="如 staff01"
                                />
                            </div>
                            <div className="form-group">
                                <label>密码 *（至少 6 位）</label>
                                <input
                                    className="form-input"
                                    type="password"
                                    value={form.password}
                                    onChange={(e) => setForm({ ...form, password: e.target.value })}
                                    placeholder="员工初始密码"
                                />
                            </div>
                            <div className="form-group">
                                <label>角色</label>
                                <select
                                    className="form-input"
                                    value={form.role}
                                    onChange={(e) => setForm({ ...form, role: e.target.value })}
                                >
                                    <option value="staff">员工（可管理业务数据）</option>
                                    <option value="admin">超级管理员（可管理账号）</option>
                                </select>
                            </div>
                        </div>
                        <div className="modal-foot">
                            <button className="btn btn-outline" onClick={() => setShowForm(false)}>
                                取消
                            </button>
                            <button className="btn" disabled={busy === 'new'} onClick={create}>
                                {busy === 'new' ? '创建中…' : '创建账号'}
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    )
}
