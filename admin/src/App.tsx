import { useState } from 'react'
import { Navigate, NavLink, Route, Routes } from 'react-router-dom'
import {
    LayoutDashboard,
    Users,
    Radio,
    MessageSquare,
    ShoppingCart,
    Tags,
    Shield,
    LogOut,
    UserCog,
    UsersRound,
    Package,
    Banknote,
    Headphones,
} from 'lucide-react'
import { clearSession, getRole, getToken, getUsername } from './api'
import Dashboard from './pages/Dashboard'
import UsersPage from './pages/Users'
import Devices from './pages/Devices'
import Messages from './pages/Messages'
import Products from './pages/Products'
import Categories from './pages/Categories'
import Accounts from './pages/Accounts'
import Teams from './pages/Teams'
import Orders from './pages/Orders'
import Withdrawals from './pages/Withdrawals'
import Support from './pages/Support'
import Login from './pages/Login'

export default function App() {
    const [authed, setAuthed] = useState(!!getToken())
    const role = getRole()
    const username = getUsername()
    const isAdmin = role === 'admin'

    if (!authed) return <Login onLogin={() => setAuthed(true)} />

    const handleLogout = () => {
        clearSession()
        setAuthed(false)
    }

    const navItems = [
        { to: '/', label: '仪表盘', icon: LayoutDashboard, end: true, adminOnly: false },
        { to: '/users', label: '用户管理', icon: Users, end: false, adminOnly: false },
        { to: '/devices', label: '设备管理', icon: Radio, end: false, adminOnly: false },
        { to: '/messages', label: '消息管理', icon: MessageSquare, end: false, adminOnly: false },
        { to: '/categories', label: '类目管理', icon: Tags, end: false, adminOnly: false },
        { to: '/products', label: '商品管理', icon: ShoppingCart, end: false, adminOnly: false },
        { to: '/orders', label: '订单管理', icon: Package, end: false, adminOnly: false },
        { to: '/withdrawals', label: '提现审核', icon: Banknote, end: false, adminOnly: false },
        { to: '/support', label: '客服工作台', icon: Headphones, end: false, adminOnly: false },
        { to: '/teams', label: '团队管理', icon: UsersRound, end: false, adminOnly: false },
        { to: '/accounts', label: '账号管理', icon: UserCog, end: false, adminOnly: true },
    ].filter((item) => !item.adminOnly || isAdmin)

    return (
        <div className="layout">
            <aside className="sidebar">
                <div className="brand">
                    <div className="brand-badge">
                        <Shield size={22} color="#fff" />
                    </div>
                    <div className="brand-text">
                        安全监护
                        <br />
                        管理工作台
                    </div>
                </div>
                <nav className="nav">
                    {navItems.map((item) => (
                        <NavLink
                            key={item.to}
                            to={item.to}
                            end={item.end}
                            className={({ isActive }) => 'nav-item' + (isActive ? ' active' : '')}
                        >
                            <item.icon size={18} />
                            <span>{item.label}</span>
                        </NavLink>
                    ))}
                </nav>
                <div className="sidebar-foot">
                    {username} · {isAdmin ? '超级管理员' : '员工'}
                </div>
            </aside>

            <main className="main">
                <header className="topbar">
                    <h1>安全监护 · 管理工作台</h1>
                    <div className="toolbar">
                        <span className={`badge ${isAdmin ? 'badge-info' : 'badge-warn'}`}>
                            {isAdmin ? '👑 超级管理员' : '👤 员工'}
                        </span>
                        <button className="btn btn-outline btn-sm" onClick={handleLogout}>
                            <LogOut size={14} />
                            退出登录
                        </button>
                    </div>
                </header>
                <div className="content">
                    <Routes>
                        <Route path="/" element={<Dashboard />} />
                        <Route path="/users" element={<UsersPage />} />
                        <Route path="/devices" element={<Devices />} />
                        <Route path="/messages" element={<Messages />} />
                        <Route path="/categories" element={<Categories />} />
                        <Route path="/products" element={<Products />} />
                        <Route path="/orders" element={<Orders />} />
                        <Route path="/withdrawals" element={<Withdrawals />} />
                        <Route path="/support" element={<Support />} />
                        <Route path="/teams" element={<Teams />} />
                        {isAdmin && <Route path="/accounts" element={<Accounts />} />}
                        <Route path="*" element={<Navigate to="/" replace />} />
                    </Routes>
                </div>
            </main>
        </div>
    )
}

