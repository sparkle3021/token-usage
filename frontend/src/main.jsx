import { createRoot } from 'react-dom/client'
import '@/index.css'
import App from '@/App.jsx'

// 禁用 WebView2 默认右键浏览器菜单
window.addEventListener('contextmenu', (e) => e.preventDefault())

// 不用 StrictMode：dev 下其 mount→unmount→mount 双渲染与 echarts 直接操作 DOM 冲突，
// 频繁切时间范围时触发 removeChild NotFoundError（生产构建本就无 StrictMode，行为一致）。
createRoot(document.getElementById('root')).render(
  <App />,
)
