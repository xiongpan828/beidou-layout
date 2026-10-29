/**
 * 全局反馈（message / modal）桥。
 *
 * antd 的静态方法 `message.xxx` / `Modal.confirm` 拿不到 ConfigProvider 上下文
 * （主题、locale、App 内的 holder 都不生效），官方推荐在组件里用 `App.useApp()`。
 * 全局请求错误处理不在组件树里，因此由 FeedbackBridge 把 App 提供的实例注入这里。
 */
import type { MessageInstance } from 'antd/es/message/interface';
import type { HookAPI as ModalHookAPI } from 'antd/es/modal/useModal';

type Feedback = { message: MessageInstance; modal: ModalHookAPI };

let current: Feedback | null = null;
const pending: Array<(feedback: Feedback) => void> = [];

/** 由 <App> 内的桥组件在挂载时调用 */
export function bindFeedback(feedback: Feedback) {
  current = feedback;
  pending.splice(0).forEach((run) => run(feedback));
}

/** 供非组件代码（如全局 request errorHandler）弹出错误提示 */
export function notifyError(content: string) {
  if (current) {
    current.message.error(content);
    return;
  }
  pending.push((feedback) => feedback.message.error(content));
}
