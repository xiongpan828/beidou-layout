import { App, Button, Descriptions, Segmented, Upload } from 'antd';
import { useState } from 'react';
import {
  ADMIN_ERROR_TEXT,
  formatTime,
  resolveErrorMessage,
  services,
  type ServiceError,
  type StorageFile,
} from '@/services';

type Scope = 'avatar' | 'attachment';

/**
 * 上传示例：两步直传。
 * 1. admin-service 签发上传地址（CreateUploadURL）；
 * 2. 浏览器把文件 PUT 到签名地址（storage-service 校验 URL 签名），成功返回 File JSON。
 * 业务表只存 key，展示时用 GetFile 换访问地址（附件是限时签名地址）。
 */
export default function StorageUploadPage() {
  const { message } = App.useApp();
  const [scope, setScope] = useState<Scope>('avatar');
  const [uploading, setUploading] = useState(false);
  const [file, setFile] = useState<StorageFile>();

  const upload = async (picked: File) => {
    setUploading(true);
    try {
      const { upload_url } = await services.storage.CreateUploadURL({ scope, filename: picked.name });
      // 直接 PUT 到签名地址：不走 umi request（错误体自己解析提示）；文件类型由服务端按扩展名定
      const res = await fetch(upload_url || '', { method: 'PUT', body: picked });
      const body = await res.json().catch(() => ({}));
      if (!res.ok) {
        message.error(resolveErrorMessage(body as ServiceError, ADMIN_ERROR_TEXT, `上传失败：${res.status}`));
        return;
      }
      setFile(body as StorageFile);
      message.success('已上传');
    } catch {
      // 签发失败由全局 errorHandler 提示
    } finally {
      setUploading(false);
    }
  };

  const refresh = async () => {
    if (!file?.key) return;
    try {
      const res = await services.storage.GetFile({ key: file.key });
      setFile(res.file);
      message.success('已刷新访问地址');
    } catch {
      // 错误提示由全局 errorHandler 处理
    }
  };

  return (
    <div className="admin-module">
      <div className="admin-module-head">
        <div className="admin-module-title">
          上传示例 <span>签发地址 → 浏览器直传 → GetFile 换访问地址</span>
        </div>
      </div>
      <div className="admin-module-body">
        <div className="admin-toolbar">
          <Segmented<Scope>
            value={scope}
            onChange={setScope}
            options={[
              { value: 'avatar', label: '头像（公有读）' },
              { value: 'attachment', label: '附件（私有，签名读）' },
            ]}
          />
          <Upload
            showUploadList={false}
            accept={scope === 'avatar' ? '.jpg,.jpeg,.png,.webp' : '.jpg,.jpeg,.png,.webp,.pdf,.txt'}
            beforeUpload={(picked) => {
              upload(picked);
              return false; // 不用 antd 自带上传
            }}
          >
            <Button type="primary" loading={uploading}>
              选择文件上传
            </Button>
          </Upload>
        </div>
        {file ? (
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start', gap: 12 }}>
            <Descriptions
              bordered
              size="small"
              column={1}
              style={{ width: '100%' }}
              items={[
                { label: 'key', children: <code>{file.key}</code> },
                { label: '文件名', children: file.filename },
                { label: '大小', children: `${file.size ?? '-'} B` },
                { label: '类型', children: file.content_type },
                { label: '上传时间', children: formatTime(file.created_at) },
                {
                  label: '访问地址',
                  children: file.url ? (
                    <a href={file.url} target="_blank" rel="noreferrer" style={{ wordBreak: 'break-all' }}>
                      {file.url}
                    </a>
                  ) : (
                    '-'
                  ),
                },
              ]}
            />
            {file.scope === 'avatar' && file.url ? (
              <img src={file.url} alt={file.filename} style={{ maxWidth: 160, maxHeight: 160, borderRadius: 8 }} />
            ) : null}
            <Button onClick={refresh}>GetFile 重新获取</Button>
          </div>
        ) : null}
      </div>
    </div>
  );
}
