import type { RequestConfig } from '@umijs/max';
import { App, ConfigProvider } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import type { ReactNode } from 'react';
import { ADMIN_ERROR_TEXT, asServiceError, resolveErrorMessage } from '@/services';
import EmptyState from '@/components/EmptyState';
import FeedbackBridge from '@/components/FeedbackBridge';
import { notifyError } from '@/feedback';
import { ADMIN_THEME } from './theme';

/** 全局主题（亮色 + 陶土橘强调，紧凑工具风） */
export function rootContainer(container: ReactNode) {
  return (
    <ConfigProvider
      locale={zhCN}
      theme={ADMIN_THEME}
      componentSize="small"
      renderEmpty={() => <EmptyState />}
    >
      <App>
        <FeedbackBridge />
        {container}
      </App>
    </ConfigProvider>
  );
}

/**
 * 全局请求错误处理：所有失败请求统一在这里处理，
 * 页面只需处理成功路径（umi 处理后仍会 reject，页面 catch 可兜底）。
 */
export const request: RequestConfig = {
  errorConfig: {
    errorHandler: (error: any, opts: any) => {
      if (opts?.skipErrorHandler) throw error;

      const serviceError = asServiceError(error);
      if (serviceError) {
        // 用户可见提示：reason 本地化 > 后端 message > 兜底
        notifyError(resolveErrorMessage(serviceError, ADMIN_ERROR_TEXT));
        // 开发者调试打印（完整 code/reason）
        console.error('[request error]', {
          code: serviceError.code,
          reason: serviceError.reason,
          message: serviceError.message,
        });
        return;
      }

      // 无结构化体：网关 502 / 超时 / 网络中断
      if (error.response) {
        notifyError(`响应异常：${error.response.status}`);
      } else if (error.request) {
        notifyError('无响应，请检查网络');
      } else {
        notifyError('请求失败，请重试');
      }
      console.error('[request error]', error);
    },
  },
};
