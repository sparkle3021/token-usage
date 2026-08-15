/**
 * antd 主题配置：亮/暗两套（shadcn 风格）。
 * 亮色：zinc 中性色 + 白底，全量 token/components 定制；
 *       按钮/输入/通知的 classNames 注入 CSS 类（见 index.css「主题增强」），
 *       等价 antd-style 写法但不引入该依赖（antd v6 兼容性存疑）。
 * 暗色：darkAlgorithm + Layout/Menu/Progress 组件覆盖。
 * 入口：<ConfigProvider {...getConfigProviderProps(dark)}>。
 */
import { theme } from 'antd';

// 亮色 token（zinc 色板）
const lightToken = {
  colorPrimary: '#262626',
  colorSuccess: '#22c55e',
  colorWarning: '#f97316',
  colorError: '#ef4444',
  colorInfo: '#262626',
  colorTextBase: '#262626',
  colorBgBase: '#ffffff',
  colorPrimaryBg: '#f5f5f5',
  colorPrimaryBgHover: '#e5e5e5',
  colorPrimaryBorder: '#d4d4d4',
  colorPrimaryBorderHover: '#a3a3a3',
  colorPrimaryHover: '#404040',
  colorPrimaryActive: '#171717',
  colorPrimaryText: '#262626',
  colorPrimaryTextHover: '#404040',
  colorPrimaryTextActive: '#171717',
  colorSuccessBg: '#f0fdf4',
  colorSuccessBgHover: '#dcfce7',
  colorSuccessBorder: '#bbf7d0',
  colorSuccessBorderHover: '#86efac',
  colorSuccessHover: '#16a34a',
  colorSuccessActive: '#15803d',
  colorSuccessText: '#16a34a',
  colorSuccessTextHover: '#16a34a',
  colorSuccessTextActive: '#15803d',
  colorWarningBg: '#fff7ed',
  colorWarningBgHover: '#fed7aa',
  colorWarningBorder: '#fdba74',
  colorWarningBorderHover: '#fb923c',
  colorWarningHover: '#ea580c',
  colorWarningActive: '#c2410c',
  colorWarningText: '#ea580c',
  colorWarningTextHover: '#ea580c',
  colorWarningTextActive: '#c2410c',
  colorErrorBg: '#fef2f2',
  colorErrorBgHover: '#fecaca',
  colorErrorBorder: '#fca5a5',
  colorErrorBorderHover: '#f87171',
  colorErrorHover: '#dc2626',
  colorErrorActive: '#b91c1c',
  colorErrorText: '#dc2626',
  colorErrorTextHover: '#dc2626',
  colorErrorTextActive: '#b91c1c',
  colorInfoBg: '#f5f5f5',
  colorInfoBgHover: '#e5e5e5',
  colorInfoBorder: '#d4d4d4',
  colorInfoBorderHover: '#a3a3a3',
  colorInfoHover: '#404040',
  colorInfoActive: '#171717',
  colorInfoText: '#262626',
  colorInfoTextHover: '#404040',
  colorInfoTextActive: '#171717',
  colorText: '#262626',
  colorTextSecondary: '#525252',
  colorTextTertiary: '#737373',
  colorTextQuaternary: '#a3a3a3',
  colorTextDisabled: '#a3a3a3',
  colorBgContainer: '#ffffff',
  colorBgElevated: '#ffffff',
  colorBgLayout: '#fafafa',
  colorBgSpotlight: 'rgba(38, 38, 38, 0.85)',
  colorBgMask: 'rgba(38, 38, 38, 0.45)',
  colorBorder: '#e5e5e5',
  colorBorderSecondary: '#f5f5f5',
  borderRadius: 10,
  borderRadiusXS: 2,
  borderRadiusSM: 6,
  borderRadiusLG: 14,
  padding: 16,
  paddingSM: 12,
  paddingLG: 24,
  margin: 16,
  marginSM: 12,
  marginLG: 24,
  boxShadow: '0 1px 3px 0 rgba(0, 0, 0, 0.1), 0 1px 2px -1px rgba(0, 0, 0, 0.1)',
  boxShadowSecondary: '0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -2px rgba(0, 0, 0, 0.1)',
};

// 亮色组件覆盖
const lightComponents = {
  Button: {
    primaryShadow: 'none',
    defaultShadow: 'none',
    dangerShadow: 'none',
    defaultBorderColor: '#e4e4e7',
    defaultColor: '#18181b',
    defaultBg: '#ffffff',
    defaultHoverBg: '#f4f4f5',
    defaultHoverBorderColor: '#d4d4d8',
    defaultHoverColor: '#18181b',
    defaultActiveBg: '#e4e4e7',
    defaultActiveBorderColor: '#d4d4d8',
    borderRadius: 6,
  },
  Input: {
    activeShadow: 'none',
    hoverBorderColor: '#a1a1aa',
    activeBorderColor: '#18181b',
    borderRadius: 6,
  },
  Select: {
    optionSelectedBg: '#f4f4f5',
    optionActiveBg: '#fafafa',
    optionSelectedFontWeight: 500,
    borderRadius: 6,
  },
  Alert: { borderRadiusLG: 8 },
  Modal: { borderRadiusLG: 12 },
  Progress: {
    circleTextColor: '#262626',
    defaultColor: '#18181b',
    remainingColor: '#f4f4f5',
  },
  Steps: { iconSize: 32 },
  Switch: { trackHeight: 22, trackMinWidth: 44, innerMinMargin: 4, innerMaxMargin: 24 },
  Checkbox: { borderRadiusSM: 4 },
  Slider: {
    trackBg: '#f4f4f5',
    trackHoverBg: '#e4e4e7',
    handleSize: 18,
    handleSizeHover: 20,
    railSize: 6,
  },
  ColorPicker: { borderRadius: 6 },
  Notification: {
    colorSuccessBg: '#f0fdf4',
    colorErrorBg: '#fef2f2',
    colorInfoBg: '#f5f5f5',
    colorWarningBg: '#fff7ed',
  },
  Layout: {
    bodyBg: '#fafafa',
    footerBg: '#fafafa',
    headerBg: '#ffffff',
    headerColor: '#18181b',
    siderBg: '#ffffff',
    triggerBg: '#f4f4f5',
    triggerColor: '#18181b',
  },
  Menu: {
    activeBarBorderWidth: 0,
    itemBg: 'transparent',
    subMenuItemBg: 'transparent',
  },
  Card: {},
  Tooltip: {},
  Radio: {},
};

// 暗色组件覆盖
const darkComponents = {
  Layout: {
    bodyBg: '#050505',
    footerBg: '#050505',
    headerBg: '#111111',
    headerColor: 'rgba(255, 255, 255, 0.88)',
    siderBg: '#050505',
    triggerBg: '#111111',
    triggerColor: 'rgba(255, 255, 255, 0.88)',
  },
  Menu: {
    darkItemBg: 'transparent',
    darkItemColor: 'rgba(255, 255, 255, 0.68)',
    darkItemHoverBg: 'rgba(255, 255, 255, 0.08)',
    darkItemHoverColor: '#fff',
    darkItemSelectedBg: 'rgba(22, 119, 255, 0.28)',
    darkItemSelectedColor: '#fff',
    darkSubMenuItemBg: 'transparent',
  },
  Button: {},
  Alert: {},
  Modal: {},
  Card: {},
  Tooltip: {},
  Checkbox: {},
  Radio: {},
  Select: {},
  Input: {},
  Switch: {},
  Progress: {
    circleTextColor: 'rgba(255, 255, 255, 0.88)',
    defaultColor: '#1677FF',
    remainingColor: 'rgba(255, 255, 255, 0.12)',
  },
  Steps: {},
  Slider: {},
  ColorPicker: {},
  Notification: {},
};

// classNames 拼接工具：过滤 falsy，空则 undefined（antd 不注入）
const cls = (...parts) => parts.filter(Boolean).join(' ') || undefined;

/**
 * 返回 ConfigProvider 完整 props（theme + 组件 classNames 注入）。
 * @param {boolean} dark 是否暗色
 */
export function getConfigProviderProps(dark) {
  if (dark) {
    return {
      theme: { algorithm: theme.darkAlgorithm, components: darkComponents },
      button: { autoInsertSpace: false },
    };
  }
  return {
    theme: { algorithm: theme.defaultAlgorithm, token: lightToken, components: lightComponents },
    button: {
      autoInsertSpace: false,
      classNames: ({ props }) => ({
        root: cls(
          props.type === 'primary' && 't-btn-primary',
          props.type === 'default' && 't-btn-default',
          props.danger && 't-btn-danger',
        ),
      }),
    },
    input: {
      classNames: ({ props }) => ({
        root: cls(props.status === 'error' && 't-input-error'),
        input: 't-input-element',
      }),
    },
    select: {
      classNames: { root: 't-select-root' },
    },
    notification: {
      classNames: {
        root: 't-notification-root',
        title: 't-notification-title',
        description: 't-notification-description',
      },
    },
  };
}
