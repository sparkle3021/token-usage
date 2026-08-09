import { Modal } from 'antd';

/**
 * 操作确认弹窗：统一交互——说明 + 取消/确认，确认后执行并 toast 反馈。
 */
export default function ConfirmDialog({ title, description, danger, confirmText, busyText, busy, onConfirm, onCancel }) {
  return (
    <Modal
      open
      onCancel={() => { if (!busy) onCancel(); }}
      onOk={onConfirm}
      okText={busy ? busyText : confirmText}
      okButtonProps={{ danger, disabled: busy, loading: busy }}
      cancelText="取消"
      cancelButtonProps={{ disabled: busy }}
      mask={{ closable: !busy }}
      keyboard={!busy}
      closable={!busy}
      centered
      title={<span className={danger ? 'text-red-600' : ''}>{title}</span>}
      width={{ xs: 416, md: 448 }}
    >
      <p className="text-xs text-muted-foreground leading-relaxed">{description}</p>
    </Modal>
  );
}
