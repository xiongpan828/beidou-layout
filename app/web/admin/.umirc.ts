import { defineConfig } from '@umijs/max';

const base = process.env.APP_BASE || '/';

export default defineConfig({
  // 生产构建由 package.json 注入 APP_BASE（/admin/），开发保持根路径
  base,
  publicPath: base,
  title: 'Beidou Layout 管理后台',
  links: [
    // 用 base 拼绝对路径，深链页面下也能取到页签图标
    { rel: 'icon', type: 'image/png', href: `${base}icons/favicon.png` },
    { rel: 'apple-touch-icon', href: `${base}icons/apple-touch-icon.png` },
  ],
  antd: {},
  request: {},
  mfsu: false,
  esbuildMinifyIIFE: true,
  // 编译 @beidou-layout/api-gen（workspace 软链的 proto 生成代码）
  extraBabelIncludes: ['@beidou-layout/api-gen'],
  // 本地开发代理（只在 pnpm dev 生效）：接口 → admin-service，文件读写 → storage-service HTTP
  proxy: {
    '/api/v1/admin': {
      target: 'http://127.0.0.1:8000',
      changeOrigin: true,
    },
    // CreateUploadURL 签发的相对地址 /api/v1/storage/file/…（URL 签名校验在 storage-service）
    '/api/v1/storage': {
      target: 'http://127.0.0.1:9301',
      changeOrigin: true,
    },
  },
  routes: [
    {
      path: '/',
      component: '@/layouts/AdminLayout',
      routes: [
        { path: '/', redirect: '/hello' },
        { path: '/hello', component: '@/pages/hello' },
        { path: '/storage/providers', component: '@/pages/storage/providers' },
        { path: '/storage/upload', component: '@/pages/storage/upload' },
        // 没有的页面回首页，不留空白页
        { path: '*', redirect: '/hello' },
      ],
    },
  ],
  npmClient: 'pnpm',
});
