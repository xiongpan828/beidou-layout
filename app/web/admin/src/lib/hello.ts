// 纯函数（无依赖），供 Hello 页面与单测使用

/** 输入的名字：去掉首尾空白，空串返回 null（不发请求） */
export function normalizeName(raw: string | null | undefined): string | null {
  const name = (raw ?? '').trim();
  return name ? name : null;
}
