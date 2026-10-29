import { App, Button, Input, Space, Table } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { normalizeName } from '@/lib/hello';
import { formatTime, services, type Greeting } from '@/services';

/** Hello：调用 admin-service 的 Hello/SayHello，并列出最近的问候记录 */
export default function HelloPage() {
  const { message } = App.useApp();
  const [name, setName] = useState('');
  const [reply, setReply] = useState('');
  const [sending, setSending] = useState(false);
  const [loading, setLoading] = useState(false);
  const [greetings, setGreetings] = useState<Greeting[]>([]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await services.hello.ListGreeting({ page: { current: 1, size: 10 }, name: undefined });
      setGreetings(res.greetings || []);
    } catch {
      // 错误提示由全局 errorHandler 处理
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const sayHello = async () => {
    const value = normalizeName(name);
    if (!value) {
      message.warning('请输入名字');
      return;
    }
    setSending(true);
    try {
      const res = await services.hello.SayHello({ name: value });
      setReply(res.greeting?.message || '');
      load();
    } catch {
      // 错误提示由全局 errorHandler 处理
    } finally {
      setSending(false);
    }
  };

  return (
    <div className="admin-module">
      <div className="admin-module-head">
        <div className="admin-module-title">
          Hello <span>admin-service → hello-service</span>
        </div>
      </div>
      <div className="admin-module-body">
        <div className="admin-toolbar">
          <Space.Compact style={{ width: 360 }}>
            <Input
              aria-label="名字"
              placeholder="输入名字"
              value={name}
              onChange={(e) => setName(e.target.value)}
              onPressEnter={sayHello}
            />
            <Button type="primary" loading={sending} onClick={sayHello}>
              打招呼
            </Button>
          </Space.Compact>
          {reply ? <span className="admin-hello-reply">{reply}</span> : null}
        </div>
        <Table<Greeting>
          rowKey={(row) => String(row.id)}
          loading={loading}
          dataSource={greetings}
          pagination={false}
          columns={[
            { title: 'ID', dataIndex: 'id', width: 80 },
            { title: '名字', dataIndex: 'name' },
            { title: '问候', dataIndex: 'message' },
            { title: '时间', dataIndex: 'created_at', width: 160, render: (v?: string) => formatTime(v) },
          ]}
        />
      </div>
    </div>
  );
}
