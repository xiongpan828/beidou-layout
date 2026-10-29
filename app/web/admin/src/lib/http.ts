/**
 * HTTP 基础设施（无 UI 依赖）：错误解析、proto 生成 client 的传输层、时间格式化。
 */

/** 后端 kratos 错误结构 {code, reason, message, metadata} */
export type ServiceError = {
  code?: number;
  reason?: string;
  message?: string;
  metadata?: Record<string, string>;
};

/** 注入给传输层的请求实现（umi request） */
export type HttpRequest = (
  url: string,
  options: { method: string; data?: unknown; headers?: Record<string, string> },
) => Promise<unknown>;

// ============ 错误 ============

/** 从 umi request（axios）抛出的 error 中解析结构化错误体 */
export function asServiceError(error: unknown): ServiceError | undefined {
  const data = (error as { response?: { data?: unknown } })?.response?.data;
  return data && typeof data === 'object' ? (data as ServiceError) : undefined;
}

/** 用户可见文案：reason 本地化 > 后端 message > 兜底 */
export function resolveErrorMessage(error: ServiceError, dict: Record<string, string> = {}, fallback = '请求失败，请重试') {
  return (error.reason && dict[error.reason]) || error.message || fallback;
}

// ============ 传输层 ============

/**
 * 适配 proto 生成 client 的 ClientTransport 接口：unary 走注入的 request，补 JSON header；
 * 接口没有流式方法，但生成器要求传输层带 serverStream / duplexStream，调用即抛错。
 */
export function createClientTransport(options: { request: HttpRequest }) {
  return {
    unary(path: string, method: string, body: string | null) {
      const headers: Record<string, string> = { Accept: 'application/json' };
      if (body) headers['Content-Type'] = 'application/json';
      // 错误统一由 app.tsx 的 request.errorConfig 处理
      return options.request(`/${path.replace(/^\/+/, '')}`, { method, data: body, headers });
    },
    serverStream(): never {
      throw new Error('流式接口未启用');
    },
    duplexStream(): never {
      throw new Error('流式接口未启用');
    },
  };
}

// ============ 格式化 ============

/** RFC3339 → YYYY-MM-DD HH:mm */
export function formatTime(value?: string): string {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '-';
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}
