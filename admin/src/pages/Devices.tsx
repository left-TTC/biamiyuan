import { useCallback, useEffect, useState } from 'react'
import { Siren, Trash2 } from 'lucide-react'
import { api } from '../api'
import { STATUS_TEXT, type Device } from '../types'

function formatTime(ts: number): string {
    const d = new Date(ts)
    const p = (n: number) => String(n).padStart(2, '0')
    return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

export default function Devices() {
    const [devices, setDevices] = useState<Device[]>([])
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState('')
    const [busy, setBusy] = useState('')

    const load = useCallback(() => {
        api.devices()
            .then((data) => {
                setDevices(data)
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

    const alarm = async (id: string) => {
        setBusy(id)
        try {
            await api.deviceAlarm(id)
            alert('已触发设备报警，消息已生成')
            load()
        } catch (err) {
            alert(err instanceof Error ? err.message : '操作失败')
        } finally {
            setBusy('')
        }
    }

    const remove = async (id: string) => {
        if (!window.confirm('确定移除该设备及其消息？')) return
        setBusy(id)
        try {
            await api.removeDevice(id)
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
            <div className="page-title">设备管理（{devices.length}）</div>
            <div className="table-wrap">
                <table>
                    <thead>
                        <tr>
                            <th>设备名称</th>
                            <th>类型</th>
                            <th>SN</th>
                            <th>状态</th>
                            <th>电量</th>
                            <th>归属用户</th>
                            <th>最后活跃</th>
                            <th>操作</th>
                        </tr>
                    </thead>
                    <tbody>
                        {devices.map((d) => (
                            <tr key={d.id}>
                                <td>{d.name}</td>
                                <td>{d.typeName || d.type}</td>
                                <td>{d.sn}</td>
                                <td>
                                    <span className={`badge badge-${d.status}`}>
                                        {STATUS_TEXT[d.status] || d.status}
                                    </span>
                                </td>
                                <td>{d.battery}%</td>
                                <td>{d.userId.slice(0, 8)}…</td>
                                <td>{formatTime(d.lastActive)}</td>
                                <td className="inline-edit">
                                    <button
                                        className="btn btn-warning btn-sm"
                                        disabled={busy === d.id}
                                        onClick={() => alarm(d.id)}
                                    >
                                        <Siren size={14} />
                                        报警
                                    </button>
                                    <button
                                        className="btn btn-danger-outline btn-sm"
                                        disabled={busy === d.id}
                                        onClick={() => remove(d.id)}
                                    >
                                        <Trash2 size={14} />
                                        移除
                                    </button>
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </table>
                {devices.length === 0 && <div className="empty">暂无设备</div>}
            </div>
        </div>
    )
}
