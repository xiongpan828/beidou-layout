// 纯函数（无依赖），供存储供应商页面与单测使用

export type ProviderType = 'minio' | 's3';

/** 供应商配置表单（proto 里 MinioConfig / S3Config 的并集；没有生成 TS 类型，这里本地定义） */
export type ProviderConfigForm = {
  endpoint?: string;
  access_key_id?: string;
  secret_access_key?: string;
  bucket?: string;
  region?: string;
  secure?: boolean;
  use_path_style?: boolean;
};

const FIELDS: Record<ProviderType, (keyof ProviderConfigForm)[]> = {
  minio: ['endpoint', 'access_key_id', 'secret_access_key', 'bucket', 'region', 'secure'],
  s3: ['endpoint', 'access_key_id', 'secret_access_key', 'bucket', 'region', 'use_path_style'],
};

/**
 * 表单 → provider.config JSON：只带该类型的字段（后端拒绝未知字段），字符串去首尾空白，
 * 空字符串不带（可选项留空；更新时密钥留空 = 沿用原值），布尔值总是带上。
 */
export function buildProviderConfig(type: ProviderType, form: ProviderConfigForm): string {
  const config: Record<string, string | boolean> = {};
  for (const field of FIELDS[type]) {
    const value = form[field];
    if (typeof value === 'boolean') config[field] = value;
    else if (field === 'secure' || field === 'use_path_style') config[field] = false;
    else if (typeof value === 'string' && value.trim()) config[field] = value.trim();
  }
  return JSON.stringify(config);
}

/** provider.config JSON → 表单（密钥后端返回空串，编辑时留空 = 不改）；非法 JSON 返回空表单 */
export function parseProviderConfig(raw: string | undefined): ProviderConfigForm {
  try {
    const value = JSON.parse(raw || '{}');
    return value && typeof value === 'object' ? { ...value, secret_access_key: '' } : {};
  } catch {
    return {};
  }
}
