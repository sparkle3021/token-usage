// antd message 全局单例：由 App.jsx 内 App.useApp() 注入。
// antd v6 静态 message.xxx 不继承 ConfigProvider 主题，必须经 App 上下文获取。
let api = null;

export function setMessageApi(a) { api = a; }
export function getMessage() { return api; }
