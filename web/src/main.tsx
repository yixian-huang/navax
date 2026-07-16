import { StrictMode } from 'react'
import './i18n'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App'

async function bootstrap() {
  // Mock 必须由开发环境变量显式开启；生产包不会安装 fetch 拦截器。
  if (import.meta.env.DEV && import.meta.env.VITE_ENABLE_API_MOCKS === 'true') {
    const { installMockApi } = await import('./api/mock-handlers')
    installMockApi()
  }

  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <App />
    </StrictMode>,
  )
}

void bootstrap()
