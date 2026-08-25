import { useCallback, useEffect, useRef, useState } from 'react'
import { Pencil, Save, X, Upload, Download, Plus, Trash2, Image as ImageIcon } from 'lucide-react'
import { api } from '../api'
import type { Category, Product } from '../types'

interface NewProductForm {
    name: string
    price: string
    originalPrice: string
    emoji: string
    parentCategory: string // 一级类目
    category: string // 二级类目
    desc: string
    sourceTeam: string // 服务来源（服务类商品）
    attributes: { name: string; values: string }[] // 内置属性（名称 + 逗号分隔的取值）
}

export default function Products() {
    const [products, setProducts] = useState<Product[]>([])
    const [categories, setCategories] = useState<Category[]>([])
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState('')
    // 编辑态：id -> { price, originalPrice, sales }
    const [editing, setEditing] = useState<Record<string, { price: string; originalPrice: string; sales: string }>>({})
    const [saving, setSaving] = useState('')
    const [busy, setBusy] = useState('')
    // Excel 导入导出
    const fileInputRef = useRef<HTMLInputElement>(null)
    const [importing, setImporting] = useState(false)
    // 新增商品
    const [showNew, setShowNew] = useState(false)
    const [newForm, setNewForm] = useState<NewProductForm>({
        name: '',
        price: '',
        originalPrice: '',
        emoji: '📦',
        parentCategory: '',
        category: '',
        desc: '',
        sourceTeam: '',
        attributes: [],
    })
    // 图片管理弹窗
    const [imageProduct, setImageProduct] = useState<Product | null>(null)
    const [imageList, setImageList] = useState<string[]>([])
    const [uploading, setUploading] = useState(false)
    const imageInputRef = useRef<HTMLInputElement>(null)

    // 两级类目辅助
    const parentCats = categories.filter((c) => !c.parentId).sort((a, b) => a.sort - b.sort)
    const childrenOf = (pid: string) => categories.filter((c) => c.parentId === pid).sort((a, b) => a.sort - b.sort)
    const catName = (id: string) => categories.find((c) => c.id === id)?.name || id || '未分类'

    const load = useCallback(() => {
        api.products()
            .then((data) => {
                setProducts(data)
                setLoading(false)
            })
            .catch((err) => {
                setError(err instanceof Error ? err.message : '加载失败')
                setLoading(false)
            })
        api.categories()
            .then(setCategories)
            .catch(() => {})
    }, [])

    useEffect(() => {
        load()
    }, [load])

    // Excel 导出（前端生成 xlsx，浏览器下载）
    const handleExport = async () => {
        setBusy('export')
        try {
            await api.exportProducts(products)
            alert('商品已导出为 Excel')
        } catch (err) {
            alert(err instanceof Error ? err.message : '导出失败')
        } finally {
            setBusy('')
        }
    }

    // Excel 导入（前端解析 → 提交服务器批量入库）
    const handleImportFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files && e.target.files[0]
        e.target.value = ''
        if (!file) return
        setImporting(true)
        try {
            const parsed = await api.parseProductsFile(file)
            if (!parsed.length) throw new Error('Excel 中无有效数据行')
            const res = await api.importProducts(parsed)
            alert(`导入完成：共 ${res.total} 行，新增 ${res.imported}，更新 ${res.updated}，失败 ${res.failed}`)
            load()
        } catch (err) {
            alert(err instanceof Error ? err.message : '导入失败')
        } finally {
            setImporting(false)
        }
    }

    // 新增商品
    const createNew = async () => {
        const price = Number(newForm.price)
        if (!newForm.name.trim() || !price || price <= 0) {
            alert('请填写商品名称和有效的价格')
            return
        }
        if (!newForm.category) {
            alert('请选择商品所属二级类目')
            return
        }
        setBusy('new')
        try {
            const attributes = newForm.attributes
                .filter((a) => a.name.trim())
                .map((a) => ({
                    name: a.name.trim(),
                    values: a.values
                        .split(/[,，]/)
                        .map((v) => v.trim())
                        .filter(Boolean),
                }))
                .filter((a) => a.values.length > 0)
            const isServiceCat = childrenOf(newForm.parentCategory).some(
                (c) => c.id === newForm.category && c.parentId === 'service'
            )
            await api.createProduct({
                name: newForm.name.trim(),
                desc: newForm.desc.trim(),
                price,
                originalPrice: Number(newForm.originalPrice) || price,
                emoji: newForm.emoji || '📦',
                category: newForm.category,
                attributes,
                sourceTeam: isServiceCat ? newForm.sourceTeam.trim() || '官方服务' : '',
            })
            setShowNew(false)
            setNewForm({
                name: '',
                price: '',
                originalPrice: '',
                emoji: '📦',
                parentCategory: '',
                category: '',
                desc: '',
                sourceTeam: '',
                attributes: [],
            })
            load()
        } catch (err) {
            alert(err instanceof Error ? err.message : '新增失败')
        } finally {
            setBusy('')
        }
    }

    // 删除商品
    const removeProduct = async (p: Product) => {
        if (!window.confirm(`确定删除商品「${p.name}」？`)) return
        setBusy(p.id)
        try {
            await api.deleteProduct(p.id)
            load()
        } catch (err) {
            alert(err instanceof Error ? err.message : '删除失败')
        } finally {
            setBusy('')
        }
    }

    // ---------- 商品图片管理 ----------

    const openImages = (p: Product) => {
        setImageProduct(p)
        setImageList(p.images && p.images.length ? [...p.images] : [])
    }

    const handleUploadImage = async (e: React.ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files && e.target.files[0]
        e.target.value = ''
        if (!file) return
        setUploading(true)
        try {
            const res = await api.uploadProductImage(file)
            setImageList((prev) => [...prev, res.url])
        } catch (err) {
            alert(err instanceof Error ? err.message : '上传失败')
        } finally {
            setUploading(false)
        }
    }

    const removeImage = (idx: number) => {
        setImageList((prev) => prev.filter((_, i) => i !== idx))
    }

    // 保存图片：通过 batch 更新商品完整数据（含 images）
    const saveImages = async () => {
        if (!imageProduct) return
        setBusy('img')
        try {
            const updated: Product = { ...imageProduct, images: imageList }
            await api.importProducts([updated])
            alert('图片已保存（第一张为列表头像）')
            setImageProduct(null)
            load()
        } catch (err) {
            alert(err instanceof Error ? err.message : '保存失败')
        } finally {
            setBusy('')
        }
    }

    const startEdit = (p: Product) => {
        setEditing((prev) => ({
            ...prev,
            [p.id]: { price: String(p.price), originalPrice: String(p.originalPrice), sales: String(p.sales) },
        }))
    }

    const cancelEdit = (id: string) => {
        setEditing((prev) => {
            const next = { ...prev }
            delete next[id]
            return next
        })
    }

    const updateField = (id: string, key: 'price' | 'originalPrice' | 'sales', value: string) => {
        setEditing((prev) => ({
            ...prev,
            [id]: { ...prev[id], [key]: value },
        }))
    }

    const save = async (p: Product) => {
        const e = editing[p.id]
        if (!e) return
        setSaving(p.id)
        try {
            await api.updateProduct(p.id, {
                price: Number(e.price),
                originalPrice: Number(e.originalPrice),
                sales: Number(e.sales),
            })
            alert('保存成功')
            cancelEdit(p.id)
            load()
        } catch (err) {
            alert(err instanceof Error ? err.message : '保存失败')
        } finally {
            setSaving('')
        }
    }

    if (loading) return <div className="loading">加载中…</div>
    if (error) return <div className="error-box">{error}</div>

    return (
        <div className="card">
            <div className="page-title">
                商品管理（{products.length}）
                <div className="toolbar" style={{ marginLeft: 'auto' }}>
                    <button className="btn btn-outline btn-sm" disabled={busy === 'export'} onClick={handleExport}>
                        <Download size={14} />
                        导出 Excel
                    </button>
                    <button
                        className="btn btn-outline btn-sm"
                        disabled={importing}
                        onClick={() => fileInputRef.current?.click()}
                    >
                        <Upload size={14} />
                        {importing ? '导入中…' : '导入 Excel'}
                    </button>
                    <button className="btn btn-sm" onClick={() => setShowNew(true)}>
                        <Plus size={14} />
                        新增商品
                    </button>
                    <input
                        ref={fileInputRef}
                        type="file"
                        accept=".xlsx,.xls"
                        style={{ display: 'none' }}
                        onChange={handleImportFile}
                    />
                </div>
            </div>
            <div className="table-wrap">
                <table>
                    <thead>
                        <tr>
                            <th>商品</th>
                            <th>分类</th>
                            <th>价格(¥)</th>
                            <th>原价(¥)</th>
                            <th>销量</th>
                            <th>操作</th>
                        </tr>
                    </thead>
                    <tbody>
                        {products.map((p) => {
                            const e = editing[p.id]
                            return (
                                <tr key={p.id}>
                                    <td>
                                        <div className="prod-cell">
                                            {p.images && p.images.length > 0 ? (
                                                <img className="prod-thumb" src={p.images[0]} alt={p.name} />
                                            ) : (
                                                <div className="prod-thumb prod-thumb-emoji">{p.emoji}</div>
                                            )}
                                            <div>
                                                <div>
                                                    {p.name}
                                                    {p.service && <span className="badge badge-service">服务</span>}
                                                </div>
                                                {p.attributes && p.attributes.length > 0 && (
                                                    <div className="prod-sub">
                                                        {p.attributes.map((a) => `${a.name}: ${a.values.join('/')}`).join(' · ')}
                                                    </div>
                                                )}
                                                {p.service && p.sourceTeam && (
                                                    <div className="prod-sub">来源：{p.sourceTeam}</div>
                                                )}
                                            </div>
                                        </div>
                                    </td>
                                    <td>{catName(p.category)}</td>
                                    {e ? (
                                        <>
                                            <td>
                                                <input
                                                    className="form-input"
                                                    style={{ width: 80, padding: '6px 8px' }}
                                                    type="number"
                                                    value={e.price}
                                                    onChange={(ev) => updateField(p.id, 'price', ev.target.value)}
                                                />
                                            </td>
                                            <td>
                                                <input
                                                    className="form-input"
                                                    style={{ width: 80, padding: '6px 8px' }}
                                                    type="number"
                                                    value={e.originalPrice}
                                                    onChange={(ev) => updateField(p.id, 'originalPrice', ev.target.value)}
                                                />
                                            </td>
                                            <td>
                                                <input
                                                    className="form-input"
                                                    style={{ width: 80, padding: '6px 8px' }}
                                                    type="number"
                                                    value={e.sales}
                                                    onChange={(ev) => updateField(p.id, 'sales', ev.target.value)}
                                                />
                                            </td>
                                        </>
                                    ) : (
                                        <>
                                            <td>{p.price}</td>
                                            <td>{p.originalPrice}</td>
                                            <td>{p.sales}</td>
                                        </>
                                    )}
                                    <td className="inline-edit">
                                        {e ? (
                                            <>
                                                <button
                                                    className="btn btn-sm"
                                                    disabled={saving === p.id}
                                                    onClick={() => save(p)}
                                                >
                                                    {saving === p.id ? (
                                                        '保存中…'
                                                    ) : (
                                                        <>
                                                            <Save size={14} />
                                                            保存
                                                        </>
                                                    )}
                                                </button>
                                                <button className="btn btn-outline btn-sm" onClick={() => cancelEdit(p.id)}>
                                                    <X size={14} />
                                                    取消
                                                </button>
                                            </>
                                        ) : (
                                            <>
                                                <button className="btn btn-outline btn-sm" onClick={() => openImages(p)}>
                                                    <ImageIcon size={14} />
                                                    图片
                                                </button>
                                                <button className="btn btn-outline btn-sm" onClick={() => startEdit(p)}>
                                                    <Pencil size={14} />
                                                    编辑
                                                </button>
                                                <button
                                                    className="btn btn-danger-outline btn-sm"
                                                    disabled={busy === p.id}
                                                    onClick={() => removeProduct(p)}
                                                >
                                                    <Trash2 size={14} />
                                                    删除
                                                </button>
                                            </>
                                        )}
                                    </td>
                                </tr>
                            )
                        })}
                    </tbody>
                </table>
                {products.length === 0 && <div className="empty">暂无商品</div>}
            </div>

            {/* 新增商品弹窗 */}
            {showNew && (
                <div className="modal-mask">
                    <div className="modal">
                        <div className="modal-head">
                            <span className="modal-title">新增商品</span>
                            <button className="modal-close" onClick={() => setShowNew(false)}>
                                <X size={18} />
                            </button>
                        </div>
                        <div className="modal-body">
                            <div className="form-group">
                                <label>商品名称 *</label>
                                <input
                                    className="form-input"
                                    value={newForm.name}
                                    onChange={(e) => setNewForm({ ...newForm, name: e.target.value })}
                                    placeholder="如：智能门锁"
                                />
                            </div>
                            <div className="form-group">
                                <label>价格（元）*</label>
                                <input
                                    className="form-input"
                                    type="number"
                                    value={newForm.price}
                                    onChange={(e) => setNewForm({ ...newForm, price: e.target.value })}
                                    placeholder="如：299"
                                />
                            </div>
                            <div className="form-group">
                                <label>原价（元）</label>
                                <input
                                    className="form-input"
                                    type="number"
                                    value={newForm.originalPrice}
                                    onChange={(e) => setNewForm({ ...newForm, originalPrice: e.target.value })}
                                    placeholder="可留空，默认等于价格"
                                />
                            </div>
                            <div className="form-group">
                                <label>一级类目</label>
                                <select
                                    className="form-input"
                                    value={newForm.parentCategory}
                                    onChange={(e) =>
                                        setNewForm({
                                            ...newForm,
                                            parentCategory: e.target.value,
                                            category: '',
                                        })
                                    }
                                >
                                    <option value="">请选择一级类目</option>
                                    {parentCats.map((c) => (
                                        <option key={c.id} value={c.id}>
                                            {c.name}
                                        </option>
                                    ))}
                                </select>
                            </div>
                            <div className="form-group">
                                <label>二级类目 *</label>
                                <select
                                    className="form-input"
                                    value={newForm.category}
                                    disabled={!newForm.parentCategory}
                                    onChange={(e) => setNewForm({ ...newForm, category: e.target.value })}
                                >
                                    <option value="">
                                        {newForm.parentCategory ? '请选择二级类目' : '请先选择一级类目'}
                                    </option>
                                    {childrenOf(newForm.parentCategory).map((c) => (
                                        <option key={c.id} value={c.id}>
                                            {c.name}
                                        </option>
                                    ))}
                                </select>
                            </div>
                            <div className="form-group">
                                <label>图标（emoji）</label>
                                <input
                                    className="form-input"
                                    value={newForm.emoji}
                                    onChange={(e) => setNewForm({ ...newForm, emoji: e.target.value })}
                                    placeholder="如：🔒"
                                />
                            </div>
                            <div className="form-group">
                                <label>描述</label>
                                <input
                                    className="form-input"
                                    value={newForm.desc}
                                    onChange={(e) => setNewForm({ ...newForm, desc: e.target.value })}
                                    placeholder="商品一句话描述"
                                />
                            </div>

                            {/* 内置属性（创建者定义，如衣服的尺码/颜色） */}
                            <div className="form-group">
                                <label>内置属性（下单时需选择，如：尺码 → S,M,L）</label>
                                <div className="attr-rows">
                                    {newForm.attributes.map((a, i) => (
                                        <div key={i} className="attr-row">
                                            <input
                                                className="form-input attr-name"
                                                placeholder="属性名，如 尺码"
                                                value={a.name}
                                                onChange={(e) => {
                                                    const arr = [...newForm.attributes]
                                                    arr[i] = { ...arr[i], name: e.target.value }
                                                    setNewForm({ ...newForm, attributes: arr })
                                                }}
                                            />
                                            <input
                                                className="form-input attr-values"
                                                placeholder="取值，逗号分隔，如 S,M,L,XL"
                                                value={a.values}
                                                onChange={(e) => {
                                                    const arr = [...newForm.attributes]
                                                    arr[i] = { ...arr[i], values: e.target.value }
                                                    setNewForm({ ...newForm, attributes: arr })
                                                }}
                                            />
                                            <button
                                                className="btn btn-danger-outline btn-xs"
                                                onClick={() =>
                                                    setNewForm({
                                                        ...newForm,
                                                        attributes: newForm.attributes.filter((_, j) => j !== i),
                                                    })
                                                }
                                            >
                                                ✕
                                            </button>
                                        </div>
                                    ))}
                                </div>
                                <button
                                    className="btn btn-outline btn-sm"
                                    onClick={() =>
                                        setNewForm({
                                            ...newForm,
                                            attributes: [...newForm.attributes, { name: '', values: '' }],
                                        })
                                    }
                                >
                                    ＋ 添加属性
                                </button>
                            </div>

                            {/* 服务来源（服务大类下） */}
                            {childrenOf(newForm.parentCategory).some((c) => c.id === newForm.category && c.parentId === 'service') && (
                                <div className="form-group">
                                    <label>服务来源（如：官方服务 / 团队名称）</label>
                                    <input
                                        className="form-input"
                                        value={newForm.sourceTeam}
                                        onChange={(e) => setNewForm({ ...newForm, sourceTeam: e.target.value })}
                                        placeholder="不填默认「官方服务」"
                                    />
                                </div>
                            )}
                        </div>
                        <div className="modal-foot">
                            <button className="btn btn-outline" onClick={() => setShowNew(false)}>
                                取消
                            </button>
                            <button className="btn" disabled={busy === 'new'} onClick={createNew}>
                                {busy === 'new' ? '保存中…' : '保存'}
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {/* 商品图片管理弹窗 */}
            {imageProduct && (
                <div className="modal-mask">
                    <div className="modal">
                        <div className="modal-head">
                            <span className="modal-title">商品图片 · {imageProduct.name}</span>
                            <button className="modal-close" onClick={() => setImageProduct(null)}>
                                <X size={18} />
                            </button>
                        </div>
                        <div className="modal-body">
                            <div className="img-grid">
                                {imageList.map((u, i) => (
                                    <div key={i} className="img-item">
                                        <img src={u} alt={`img-${i}`} className="img-thumb" />
                                        {i === 0 && <span className="img-main">头像</span>}
                                        <button className="img-del" onClick={() => removeImage(i)}>
                                            <Trash2 size={12} />
                                        </button>
                                    </div>
                                ))}
                                <button
                                    className="img-add"
                                    disabled={uploading}
                                    onClick={() => imageInputRef.current?.click()}
                                >
                                    <Upload size={20} />
                                    {uploading ? '上传中…' : '上传图片'}
                                </button>
                                <input
                                    ref={imageInputRef}
                                    type="file"
                                    accept="image/*"
                                    style={{ display: 'none' }}
                                    onChange={handleUploadImage}
                                />
                            </div>
                            <p className="img-tip">第一张图将作为商品列表头像；详情页可左右滑动查看全部图片</p>
                        </div>
                        <div className="modal-foot">
                            <button className="btn btn-outline" onClick={() => setImageProduct(null)}>
                                取消
                            </button>
                            <button className="btn" disabled={busy === 'img' || uploading} onClick={saveImages}>
                                {busy === 'img' ? '保存中…' : '保存图片'}
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    )
}
