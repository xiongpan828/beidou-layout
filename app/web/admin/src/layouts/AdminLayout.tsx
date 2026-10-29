import { history, Outlet, useLocation } from '@umijs/max';

type NavItem = { path: string; label: string };

// 顶栏导航：新增页面时在 .umirc.ts 加路由，再在这里加一项
const NAV: NavItem[] = [
  { path: '/hello', label: 'Hello' },
  { path: '/storage/providers', label: '存储供应商' },
  { path: '/storage/upload', label: '上传示例' },
];

/** B 端 PC 外壳：顶栏（品牌 + 导航）+ 内容区 */
export default function AdminLayout() {
  const { pathname } = useLocation();

  return (
    <div className="admin-layout">
      <header className="admin-header">
        <div className="admin-brand">
          <span className="admin-brand-mark" aria-hidden="true" />
          <span className="admin-brand-text">
            <span className="admin-brand-name">Beidou Layout</span>
            <span className="admin-brand-sub">管理后台</span>
          </span>
        </div>
        <nav className="admin-big-tabs">
          {NAV.map((item) => (
            <button
              key={item.path}
              type="button"
              className={`admin-big-tab ${pathname === item.path ? 'active' : ''}`}
              onClick={() => history.push(item.path)}
            >
              <strong>{item.label}</strong>
            </button>
          ))}
        </nav>
      </header>
      <main className="admin-main">
        <Outlet />
      </main>
    </div>
  );
}
