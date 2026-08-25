import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// 工作台开发服务器：默认 5173，代理 /api 到 Go 服务器
export default defineConfig({
    plugins: [react()],
    server: {
        port: 5173,
        proxy: {
            '/api': {
                target: 'http://127.0.0.1:8080',
                changeOrigin: true,
            },
            '/uploads': {
                target: 'http://127.0.0.1:8080',
                changeOrigin: true,
            },
        },
    },
})
