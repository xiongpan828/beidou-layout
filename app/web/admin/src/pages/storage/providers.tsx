import { App, Button, Form, Input, Modal, Segmented, Space, Switch, Table, Tag } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { buildProviderConfig, parseProviderConfig, type ProviderConfigForm, type ProviderType } from '@/lib/storage';
import { formatTime, services, type StorageProvider } from '@/services';

type ProviderForm = { name?: string; code?: string; type: ProviderType } & ProviderConfigForm;

/** 存储供应商（MinIO / S3 兼容）：服务配置里的 scope.provider_code 引用这里的 code。密钥不回显，编辑时留空 = 不改 */
export default function StorageProvidersPage() {
  const { message, modal } = App.useApp();
  const [form] = Form.useForm<ProviderForm>();
  const type = Form.useWatch('type', form) ?? 'minio';
  const [providers, setProviders] = useState<StorageProvider[]>([]);
  const [loading, setLoading] = useState(false);
  // undefined = 弹窗关闭，null = 新建，否则编辑这一条
  const [editing, setEditing] = useState<StorageProvider | null | undefined>(undefined);
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await services.storage.ListProvider({});
      setProviders(res.providers || []);
    } catch {
      // 错误提示由全局 errorHandler 处理
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const open = (row: StorageProvider | null) => {
    form.resetFields();
    form.setFieldsValue(
      row
        ? { name: row.name, code: row.code, type: row.type as ProviderType, ...parseProviderConfig(row.config) }
        : { type: 'minio', secure: false, use_path_style: false },
    );
    setEditing(row);
  };

  const submit = async ({ name, code, type: providerType, ...config }: ProviderForm) => {
    setSaving(true);
    try {
      const json = buildProviderConfig(providerType, config);
      if (editing) {
        await services.storage.UpdateProvider({ code: editing.code, name, config: json });
      } else {
        await services.storage.CreateProvider({ name, code, type: providerType, config: json });
      }
      message.success('已保存');
      setEditing(undefined);
      load();
    } catch {
      // 错误提示由全局 errorHandler 处理
    } finally {
      setSaving(false);
    }
  };

  const remove = (row: StorageProvider) =>
    modal.confirm({
      title: `删除供应商「${row.name}」？`,
      content: '被存储场景或已有文件引用的供应商不能删除。',
      okButtonProps: { danger: true },
      onOk: async () => {
        await services.storage.DeleteProvider({ code: row.code });
        message.success('已删除');
        load();
      },
    });

  return (
    <div className="admin-module">
      <div className="admin-module-head">
        <div className="admin-module-title">
          存储供应商 <span>scope.provider_code 引用这里的编码</span>
        </div>
        <Button type="primary" onClick={() => open(null)}>
          新建供应商
        </Button>
      </div>
      <div className="admin-module-body">
        <Table<StorageProvider>
          rowKey={(row) => String(row.code)}
          loading={loading}
          dataSource={providers}
          pagination={false}
          columns={[
            { title: '名称', dataIndex: 'name' },
            { title: '编码', dataIndex: 'code', render: (v?: string) => <code>{v}</code> },
            { title: '类型', dataIndex: 'type', width: 90, render: (v?: string) => <Tag>{v}</Tag> },
            { title: '桶', key: 'bucket', render: (_, row) => parseProviderConfig(row.config).bucket || '-' },
            { title: '更新时间', dataIndex: 'updated_at', width: 150, render: (v?: string) => formatTime(v) },
            {
              title: '操作',
              key: 'actions',
              width: 130,
              render: (_, row) => (
                <Space size={4}>
                  <Button type="link" size="small" onClick={() => open(row)}>
                    编辑
                  </Button>
                  <Button type="link" size="small" danger onClick={() => remove(row)}>
                    删除
                  </Button>
                </Space>
              ),
            },
          ]}
        />
      </div>

      <Modal
        title={editing ? `编辑供应商：${editing.code}` : '新建供应商'}
        open={editing !== undefined}
        onOk={form.submit}
        onCancel={() => setEditing(undefined)}
        confirmLoading={saving}
        forceRender
      >
        <Form form={form} layout="vertical" requiredMark={false} onFinish={submit}>
          <Form.Item name="name" label="名称" rules={[{ required: true, whitespace: true, max: 100, message: '请输入名称' }]}>
            <Input />
          </Form.Item>
          <Form.Item
            name="code"
            label="编码"
            extra="创建后不能改"
            rules={[{ required: true, pattern: /^[a-z0-9][a-z0-9-]{0,99}$/, message: '小写字母 / 数字 / 中划线，字母或数字开头' }]}
          >
            <Input disabled={!!editing} placeholder="default" />
          </Form.Item>
          <Form.Item name="type" label="类型" extra={editing ? '类型不能改' : undefined}>
            <Segmented<ProviderType> disabled={!!editing} options={[{ value: 'minio', label: 'MinIO' }, { value: 's3', label: 'S3 / 兼容存储' }]} />
          </Form.Item>
          {type === 'minio' ? (
            <Form.Item
              name="endpoint"
              label="Endpoint"
              rules={[{ required: true, pattern: /^(?!\w+:\/\/)[^\s/]+$/, message: 'host:port，不带 http(s)://' }]}
            >
              <Input placeholder="127.0.0.1:9000" />
            </Form.Item>
          ) : (
            <Form.Item
              name="endpoint"
              label="Endpoint"
              extra="AWS 留空；OSS / COS / R2 等兼容存储填完整地址"
              rules={[{ pattern: /^https:\/\/\S+$/, message: '以 https:// 开头的完整地址' }]}
            >
              <Input placeholder="https://…" />
            </Form.Item>
          )}
          <Form.Item name="access_key_id" label="Access Key ID" rules={[{ required: true, whitespace: true, message: '请输入 Access Key ID' }]}>
            <Input autoComplete="off" />
          </Form.Item>
          <Form.Item
            name="secret_access_key"
            label="Secret Access Key"
            extra={editing ? '不回显；留空 = 沿用原密钥' : undefined}
            rules={editing ? [] : [{ required: true, whitespace: true, message: '请输入 Secret Access Key' }]}
          >
            <Input.Password autoComplete="new-password" placeholder={editing ? '留空不修改' : undefined} />
          </Form.Item>
          <Form.Item name="bucket" label="Bucket" rules={[{ required: true, whitespace: true, message: '请输入 Bucket' }]}>
            <Input />
          </Form.Item>
          <Form.Item
            name="region"
            label="Region"
            rules={type === 's3' ? [{ required: true, whitespace: true, message: 'S3 必须填 Region' }] : []}
          >
            <Input placeholder={type === 's3' ? 'us-east-1' : '可选'} />
          </Form.Item>
          {type === 'minio' ? (
            <Form.Item name="secure" label="HTTPS" valuePropName="checked">
              <Switch />
            </Form.Item>
          ) : (
            <Form.Item name="use_path_style" label="Path-style 访问" valuePropName="checked" extra="多数兼容存储需要打开">
              <Switch />
            </Form.Item>
          )}
        </Form>
      </Modal>
    </div>
  );
}
