import {
  createHelloClient,
  type servicehello_service_Greeting,
} from '@beidou-layout/api-gen/service/admin_service/admin_hello';
import {
  createStorageClient,
  type servicestorage_service_File,
  type servicestorage_service_Provider,
} from '@beidou-layout/api-gen/service/admin_service/admin_storage';
import { request } from '@umijs/max';
import { createClientTransport, type HttpRequest } from '@/lib/http';

// ============ services（B 端业务 client，应用内维护） ============

const transport = createClientTransport({
  // umi request 的调用签名（method/data/headers）与 HttpRequest 兼容，仅做类型适配
  request: request as unknown as HttpRequest,
});

export const services = {
  hello: createHelloClient(transport),
  storage: createStorageClient(transport),
};

// ============ 错误文案（后端 message 为英文开发文案，按 reason 本地化） ============

export const ADMIN_ERROR_TEXT: Record<string, string> = {
  VALIDATOR: '请检查输入内容',
  PROVIDER_CODE_EXISTS: '供应商编码已存在',
  PROVIDER_CONFIG_INVALID: '供应商配置不正确，请检查各项',
  PROVIDER_IN_USE: '供应商正在被使用（存储场景或已有文件），不能删除',
  SIGNATURE_INVALID: '上传地址无效或已过期，请重新选择文件',
  FILE_ALREADY_UPLOADED: '这个上传地址已经用过了，请重新选择文件',
  FILE_EXT_NOT_ALLOWED: '不支持这种文件类型',
  FILE_NOT_FOUND: '文件不存在或还没上传完成',
};

// ============ 类型（页面从 @/services 统一 import） ============

export type Greeting = servicehello_service_Greeting;
export type StorageProvider = servicestorage_service_Provider;
export type StorageFile = servicestorage_service_File;

export { asServiceError, resolveErrorMessage, formatTime, type ServiceError } from '@/lib/http';
