import { useCallback, useEffect, useState } from 'react'
import { Pencil, Plus, Trash2, X } from 'lucide-react'
import { api } from '../api'
import type { Category } from '../types'

interface FormState {
    id?: string
    name: string
    sort: number
    isService: boolean
}

const emptyForm: FormState = { name: '', sort: 0, isService: false }

export default function Categories() {
    const [list, setList] = useState<Category[]>([])
    const [selected, setSelected] = useState<Category | null>(null) // 当前选中的一级类目
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState('')
    const [showForm, setShowForm] = useState(false)
    const [form, setForm] = useState<FormState>(emptyForm)
    const [formParent, setFormParent] = useState('') // 表单所属一级类目（新建二级类目时使用）
    const [busy, setBusy] = useState(false)

    const load = useCallback(() => {
        api.categories()
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

    const parents = list
        .filter((c) => !c.parentId)
        .sort((a, b) => a.sort - b.sort || a.id.localeCompare(b.id))
    const childrenOf = (pid: string) =>
        list
            .filter((c) => c.parentId === pid)
            .sort((a, b) => a.sort - b.sort || a.id.localeCompare(b.id))
    const selectedId = selected ? selected.id : parents.length ? parents[0].id : ''

    const openCreateParent = () => {
        setForm({ name: '', sort: 0, isService: false })
        setFormParent('')
        setShowForm(true)
    }

    const openCreateChild = () => {
        if (!selectedId) return
        setForm({ name: '', sort: 0, isService: false })
        setFormParent(selectedId)
        setShowForm(true)
    }

    const openEdit = (c: Category) => {
        setForm({ id: c.id, name: c.name, sort: c.sort, isService: !!c.isService })
        setFormParent(c.parentId)
        setShowForm(true)
    }

    const save = async () => {
        if (!form.name.trim()) {
            alert('请输入类目名称')
            return
        }
        setBusy(true)
        try {
            // 服务大类只能是已存在的一级类目标记（新建一级类目勾选"服务大类"也允许）
            const payload: Record<string, unknown> = {
                name: form.name.trim(),
                sort: form.sort,
                parentId: formParent,
            }
            if (!formParent) payload.isService = form.isService
            if (form.id) {
                await api.updateCategory(form.id, payload)
            } else {
                await api.createCategory(payload)
            }
            setShowForm(false)
            load()
        } catch (err) {
            alert(err instanceof Error ? err.message : '保存失败')
        } finally {
            setBusy(false)
        }
    }

    const remove = async (c: Category) => {
        if (c.id === 'service') {
            alert('「服务」为固定大类，不可删除')
            return
        }
        const tip = c.parentId
            ? `确定删除二级类目「${c.name}」？`
            : `确定删除一级类目「${c.name}」？其下的二级类目将一并删除`
        if (!window.confirm(tip)) return
        setBusy(true)
        try {
            await api.deleteCategory(c.id)
            if (selectedId === c.id) setSelected(null)
            load()
        } catch (err) {
            alert(err instanceof Error ? err.message : '删除失败')
        } finally {
            setBusy(false)
        }
    }

    if (loading) return <div className="loading">加载中…</div>
    if (error) return <div className="error-box">{error}</div>

    const currentChildren = childrenOf(selectedId)

    return (
        <div className="card">
            <div className="page-title">类目管理（一级类目 → 二级类目）</div>

            <div className="cat-two-pane">
                {/* 左侧：一级类目 */}
                <div className="cat-pane">
                    <div className="cat-pane-head">
                        <span>一级类目（{parents.length}）</span>
                        <button className="btn btn-sm" onClick={openCreateParent}>
                            <Plus size={14} /> 新增
                        </button>
                    </div>
                    {parents.length === 0 && <div className="empty">暂无一级类目，点击右上角新增</div>}
                    {parents.map((c) => (
                        <div
                            key={c.id}
                            className={`cat-item ${c.id === selectedId ? 'active' : ''}`}
                            onClick={() => setSelected(c)}
                        >
                            <span className="cat-name">
                                {c.name}
                                {c.id === 'service' && <span className="badge badge-service">服务</span>}
                            </span>
                            <span className="cat-ops">
                                <button
                                    className="btn btn-outline btn-xs"
                                    onClick={(e) => {
                                        e.stopPropagation()
                                        openEdit(c)
                                    }}
                                >
                                    <Pencil size={12} />
                                </button>
                                <button
                                    className="btn btn-danger-outline btn-xs"
                                    onClick={(e) => {
                                        e.stopPropagation()
                                        remove(c)
                                    }}
                                >
                                    <Trash2 size={12} />
                                </button>
                            </span>
                        </div>
                    ))}
                </div>

                {/* 右侧：二级类目 */}
                <div className="cat-pane">
                    <div className="cat-pane-head">
                        <span>二级类目{selectedId ? `（${currentChildren.length}）` : ''}</span>
                        <button className="btn btn-sm" disabled={!selectedId} onClick={openCreateChild}>
                            <Plus size={14} /> 新增
                        </button>
                    </div>
                    {!selectedId ? (
                        <div className="empty">暂无一级类目，请先创建</div>
                    ) : currentChildren.length === 0 ? (
                        <div className="empty">该一级类目下暂无二级类目</div>
                    ) : (
                        currentChildren.map((c) => (
                            <div key={c.id} className="cat-item">
                                <span className="cat-name">{c.name}</span>
                                <span className="cat-ops">
                                    <button className="btn btn-outline btn-xs" onClick={() => openEdit(c)}>
                                        <Pencil size={12} />
                                    </button>
                                    <button className="btn btn-danger-outline btn-xs" onClick={() => remove(c)}>
                                        <Trash2 size={12} />
                                    </button>
                                </span>
                            </div>
                        ))
                    )}
                </div>
            </div>

            {/* 新增/编辑弹窗 */}
            {showForm && (
                <div className="modal-mask">
                    <div className="modal">
                        <div className="modal-head">
                            <span className="modal-title">
                                {form.id ? '编辑' : '新增'}
                                {formParent ? '二级类目' : '一级类目'}
                            </span>
                            <button className="modal-close" onClick={() => setShowForm(false)}>
                                <X size={18} />
                            </button>
                        </div>
                        <div className="modal-body">
                            <div className="form-group">
                                <label>类目名称 *</label>
                                <input
                                    className="form-input"
                                    value={form.name}
                                    onChange={(e) => setForm({ ...form, name: e.target.value })}
                                    placeholder={formParent ? '如：云台摄像头' : '如：智能家居'}
                                />
                            </div>
                            <div className="form-group">
                                <label>排序（数字越小越靠前）</label>
                                <input
                                    className="form-input"
                                    type="number"
                                    value={form.sort}
                                    onChange={(e) => setForm({ ...form, sort: Number(e.target.value) || 0 })}
                                />
                            </div>
                            {!formParent && (
                                <div className="form-group form-check">
                                    <label className="check-row">
                                        <input
                                            type="checkbox"
                                            checked={form.isService}
                                            onChange={(e) => setForm({ ...form, isService: e.target.checked })}
                                        />
                                        <span>服务大类（该大类下商品为服务类商品，可由团队发布）</span>
                                    </label>
                                </div>
                            )}
                            {formParent && (
                                <div className="form-group">
                                    <label>所属一级类目</label>
                                    <input
                                        className="form-input"
                                        value={parents.find((p) => p.id === formParent)?.name || ''}
                                        disabled
                                    />
                                </div>
                            )}
                        </div>
                        <div className="modal-foot">
                            <button className="btn btn-outline" onClick={() => setShowForm(false)}>
                                取消
                            </button>
                            <button className="btn" disabled={busy} onClick={save}>
                                {busy ? '保存中…' : '保存'}
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    )
}
