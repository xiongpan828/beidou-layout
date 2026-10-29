import { App } from 'antd';
import { useEffect } from 'react';
import { bindFeedback } from '@/feedback';

/** 把 App 上下文的 message/modal 实例交给非组件代码使用（见 src/feedback.ts） */
export default function FeedbackBridge() {
  const app = App.useApp();

  useEffect(() => {
    bindFeedback(app);
  }, [app]);

  return null;
}
