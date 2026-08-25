import { FormEvent, useState } from 'react'
import { ShieldCheck, KeyRound, LogIn, User } from 'lucide-react'
import { api } from '../api'

export default function Login({ onLogin }: { onLogin: () => void }) {
    const [username, setUsername] = useState('')
    const [password, setPassword] = useState('')
    const [error, setError] = useState('')
    const [loading, setLoading] = useState(false)

    const handleSubmit = async (e: FormEvent) => {
        e.preventDefault()
        if (!username.trim() || !password) {
            setError('请输入用户名和密码')
            return
        }
        setLoading(true)
        setError('')
        try {
            await api.adminLogin(username.trim(), password)
            onLogin()
        } catch (err) {
            setError(err instanceof Error ? err.message : '登录失败')
        } finally {
            setLoading(false)
        }
    }

    return (
        <div className="login-page">
            <form className="login-card" onSubmit={handleSubmit}>
                <div className="login-icon">
                    <ShieldCheck size={52} color="#ffffff" strokeWidth={1.5} />
                </div>
                <div className="login-title">安全监护工作台</div>
                <div className="login-sub">管理后台账号登录</div>
                {error && <div className="error-box">{error}</div>}
                <div className="form-group">
                    <label>用户名</label>
                    <div className="input-with-icon">
                        <User size={16} color="#9aa1ab" />
                        <input
                            className="form-input"
                            value={username}
                            onChange={(e) => setUsername(e.target.value)}
                            placeholder="如 admin"
                            autoFocus
                        />
                    </div>
                </div>
                <div className="form-group">
                    <label>密码</label>
                    <div className="input-with-icon">
                        <KeyRound size={16} color="#9aa1ab" />
                        <input
                            className="form-input"
                            type="password"
                            value={password}
                            onChange={(e) => setPassword(e.target.value)}
                            placeholder="请输入密码"
                        />
                    </div>
                </div>
                <button className="btn login-btn" type="submit" disabled={loading}>
                    <LogIn size={16} />
                    {loading ? '登录中…' : '登 录'}
                </button>
                <div className="login-tip">初始账号 admin / admin123 · 由超级管理员创建员工账号</div>
            </form>
        </div>
    )
}

