import { useCallback, useEffect, useState } from 'react'
import { Trash2, Users as UsersIcon } from 'lucide-react'
import { api } from '../api'
import type { Team } from '../types'

export default function Teams() {
    const [teams, setTeams] = useState<Team[]>([])
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState('')
    const [busy, setBusy] = useState('')

    const load = useCallback(() => {
        api.teams()
            .then((data) => {
                setTeams(data)
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

    const remove = async (t: Team) => {
        if (!window.confirm(`确定解散团队「${t.name}」？团队成员将被移出`)) return
        setBusy(t.id)
        try {
            await api.removeTeam(t.id)
            load()
        } catch (err) {
            alert(err instanceof Error ? err.message : '解散失败')
        } finally {
            setBusy('')
        }
    }

    const fmt = (n: number) => (n || 0).toLocaleString('zh-CN', { maximumFractionDigits: 2 })

    if (loading) return <div className="loading">加载中…</div>
    if (error) return <div className="error-box">{error}</div>

    return (
        <div className="page">
            <div className="page-head">
                <div>
                    <div className="page-title">团队管理</div>
                    <div className="page-sub">团队可发布服务类商品（邀请人数 &gt; 2 或所在团队经营金额 &gt; 1w 可创建团队）</div>
                </div>
            </div>

            {teams.length === 0 ? (
                <div className="empty">
                    <UsersIcon size={48} color="#c0c6cf" />
                    <div className="empty-text">暂无团队</div>
                </div>
            ) : (
                <div className="table-wrap">
                    <table>
                        <thead>
                            <tr>
                                <th>团队名称</th>
                                <th>队长</th>
                                <th>成员数</th>
                                <th>经营金额(¥)</th>
                                <th>金库(¥)</th>
                                <th>创建时间</th>
                                <th>操作</th>
                            </tr>
                        </thead>
                        <tbody>
                            {teams.map((t) => (
                                <tr key={t.id}>
                                    <td>
                                        <span className="badge badge-info">团队</span> {t.name}
                                    </td>
                                    <td>{t.ownerName || t.ownerPhone}</td>
                                    <td>{t.members ? t.members.length : 1} 人</td>
                                    <td>{fmt(t.businessAmount)}</td>
                                    <td>
                                        <span className="badge badge-warning">{fmt(t.treasury)}</span>
                                    </td>
                                    <td>{new Date(t.createdAt).toLocaleString('zh-CN')}</td>
                                    <td className="inline-edit">
                                        <button
                                            className="btn btn-danger-outline btn-xs"
                                            disabled={busy === t.id}
                                            onClick={() => remove(t)}
                                        >
                                            <Trash2 size={12} /> 解散
                                        </button>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}

            {teams.some((t) => t.members && t.members.length > 1) && (
                <div className="card" style={{ marginTop: 16 }}>
                    <div className="card-title">成员明细</div>
                    {teams.map(
                        (t) =>
                            t.members &&
                            t.members.length > 1 && (
                                <div key={t.id} style={{ margin: '8px 0' }}>
                                    <b>{t.name}</b>：{t.members.map((m) => m.nickName || m.phone).join('、')}
                                </div>
                            )
                    )}
                </div>
            )}
        </div>
    )
}
