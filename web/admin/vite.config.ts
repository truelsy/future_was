import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// Vite 빌드 산출물을 Go embed 디렉토리로 직접 출력.
// dev 시에는 /admin/* API 요청을 Go 서버(8089)로 프록시.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  base: '/admin/ui/',
  build: {
    outDir: '../../internal/admin_ui/dist',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: {
      '/admin': {
        target: 'http://localhost:8089',
        changeOrigin: false,
      },
    },
  },
})
