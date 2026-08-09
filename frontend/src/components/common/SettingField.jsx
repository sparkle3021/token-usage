/**
 * 设置项三段式模板：小标题 + 描述 + 内容控件。
 * 全页面设置项统一经此组件呈现，间距与样式单点控制。
 */
export default function SettingField({ title, description, children }) {
  return (
    <div className="space-y-1.5">
      <h4 className="text-sm font-medium">{title}</h4>
      <p className="text-xs text-muted-foreground leading-relaxed">{description}</p>
      <div className="mt-2">{children}</div>
    </div>
  );
}
