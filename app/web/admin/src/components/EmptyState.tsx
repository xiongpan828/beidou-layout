import type { ReactNode } from 'react';

/** 空状态：线框图标 + 一句提示（antd ConfigProvider.renderEmpty 用；样式见 global.css 的 admin-empty） */
export default function EmptyState({ text = '这里还空着' }: { text?: ReactNode }) {
  return (
    <div className="admin-empty">
      <svg className="admin-empty-icon" width={64} height={48} viewBox="0 0 64 48" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
        <rect x="6" y="10" width="52" height="32" rx="4" strokeDasharray="4 3" />
        <path d="M6 20h52" />
      </svg>
      <span className="admin-empty-text">{text}</span>
    </div>
  );
}
