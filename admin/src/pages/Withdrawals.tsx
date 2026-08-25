import { useCallback, useEffect, useState } from 'react'
import { Banknote, CheckCircle2, XCircle } from 'lucide-react'
import { api } from '../api'
import type { Withdrawal } from '../types'

const METHOD_TEXT: Record<string, string> = {
    wechat: '微信零钱',
    alipay: '支付宝',
    bank: '银行卡',
}

const STATUS_TEXT: Record<string, string> = {
    processing: '处理中',
    done: '已到账',
    failed: '已驳回',
}

export default function Withdrawals() {
    const [list, setList] = useState<Withdrawal[]>([])
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState('')
    const [busy, setBusy] = useState('')

    const load = useCallback(() => {
        api.withdrawals()
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

    const act = async (id: string, type: 'complete' | 'fail') => {
        const w = list.find((x) => x.id === id)
        if (!w) return
        const ok =
            type === 'complete'
                ? window.confirm(`确认提现打款 ¥${w.amount}？收款账户：${w.account || '（未绑定）'}，真实模式下将通过微信转账到零钱`)
                : window.confirm(`确定驳回 ¥${w.amount} 的提现申请？将自动退回用户余额`)
        if (!ok) return
        setBusy(id)
        try {
            if (type === 'complete') await api.completeWithdrawal(id)
            else await api.failWithdrawal(id)
            load()
        } catch (err) {
            alert(err instanceof Error ? err.message : '操作失败')
        } finally {
            setBusy('')
        }
    }

    const fmt = (n?: number) => (n || 0).toLocaleString('zh-CN', { maximumFractionDigits: 2 })
    const statusClass = (s: string) =>
        s === 'done' ? 'badge badge-success' : s === 'failed' ? 'badge badge-danger' : 'badge badge-warning'

    return (
        <div className="page">
            <div className="page-head">
                <div>
                    <div className="page-title">提现审核</div>
                    <div className="page-sub">提现由服务器处理，打款绑定 .env 配置的企业银行卡号；驳回自动退款到用户余额</div>
                </div>
            </div>

            {loading ? (
                <div className="loading">加载中…</div>
            ) : error ? (
                <div className="error-box">{error}</div>
            ) : list.length === 0 ? (
                <div className="empty">
                    <Banknote size={48} color="#c0c6cf" />
                    <div className="empty-text">暂无提现申请</div>
                </div>
            ) : (
                <div className="table-wrap">
                    <table>
                        <thead>
                            <tr>
                                <th>用户</th>
                                <th>金额</th>
                                <th>方式</th>
                                <th>收款账户</th>
                                <th>企业银行卡号</th>
                                <th>状态</th>
                                <th>申请时间</th>
                                <th>操作</th>
                            </tr>
                        </thead>
                        <tbody>
                            {list.map((w) => (
                                <tr key={w.id}>
                                    <td>{w.userId}</td>
                                    <td>¥{fmt(w.amount)}</td>
                                    <td>{METHOD_TEXT[w.method] || w.method}</td>
                                    <td>{w.account || '—'}</td>
                                    <td className="mono">{w.bankCardNo || '—'}</td>
                                    <td>
                                        <span className={statusClass(w.status)}>{STATUS_TEXT[w.status] || w.status}</span>
                                    </td>
                                    <td>{new Date(w.applyTime).toLocaleString('zh-CN')}</td>
                                    <td className="inline-edit">
                                        {w.status === 'processing' && (
                                            <>
                                                <button
                                                    className="btn btn-outline btn-sm"
                                                    disabled={busy === w.id}
                                                    onClick={() => act(w.id, 'complete')}
                                                >
                                                    <CheckCircle2 size={12} /> 打款完成
                                                </button>
                                                <button
                                                    className="btn btn-danger-outline btn-sm"
                                                    disabled={busy === w.id}
                                                    onClick={() => act(w.id, 'fail')}
                                                >
                                                    <XCircle size={12} /> 驳回
                                                </button>
                                            </>
                                        )}
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}
        </div>
    )
}
