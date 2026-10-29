# 回归测试

面向当前架构的轻量回归套件：

- **UI**（`tests/e2e`）：Playwright（Chromium），覆盖运营后台的 Hello 示例页与存储页面。
- **API**（`tests/api`）：Go 集成测试（`-tags=integration`），经网关验证 C 端手写 BFF、B 端纯代理、参数校验、对象存储上传往返（签发 → PUT → 读回）。
- **迁移一致性**（`app/service/*/migrate/migrate_test.go`）：空库跑全部迁移两次，再校验 gorm 模型与迁移建出的表结构一致。
- **前端单测**：`app/web/admin/tests/*.test.ts`（`node:test`，只测纯函数），`pnpm --filter @beidou-layout/web-admin test`。

## 1. UI 测试

配置在 `tests/e2e/playwright.config.ts`（Desktop Chrome 设备参数 + Playwright 自带的 Chromium），串行（`workers: 1`）、失败时截图 + trace，报告输出到 `tests/e2e/playwright-report`。

```bash
pnpm test:e2e:install   # 首次：安装 Playwright 的 Chromium 及系统依赖

# 本地 dev：make migrate，起 hello / storage / admin 服务（make run-hello / run-storage / run-admin）+ pnpm dev:admin
pnpm test:e2e           # = playwright test --config tests/e2e/playwright.config.ts

# 远程
ADMIN_WEB_URL=https://<域名>/admin/ pnpm test:e2e
```

环境变量（URL 末尾会自动补 `/`）：

| 变量 | 默认 |
|---|---|
| `ADMIN_WEB_URL` | `http://127.0.0.1:3000/` |

覆盖点：

- 首页默认进入 `/hello`，页面显示品牌名。
- 输入名字点「打招呼」，显示后端返回的 `Hello, <name>!`（会写一条问候记录）。
- 「存储供应商」（有「新建供应商」）和「上传示例」页面可打开（只验证页面可用，不真的上传）。

## 2. API 测试

```bash
# 需要经网关的统一入口（/api/v1/admin、/api/v1/consumer、/api/v1/storage 在同一个地址下），一般对部署好的开发环境跑
make test-api                                                     # 默认 TEST_BASE_URL=http://127.0.0.1:9080
TEST_BASE_URL=https://<域名> go test -tags=integration ./tests/api -count=1 -v
```

会写问候记录、上传测试文件，不要对生产库跑。存储用例需要 code 为 `default` 的供应商：没有时设 `TEST_STORAGE_PROVIDER_CONFIG`（MinIO 配置 JSON，`TEST_STORAGE_PROVIDER_TYPE` 默认 `minio`）由测试创建，否则跳过。

## 3. 迁移一致性测试

```bash
TEST_HELLO_DSN="host=127.0.0.1 user=postgres password=postgres dbname=hello_test sslmode=disable" \
TEST_STORAGE_DSN="host=127.0.0.1 user=postgres password=postgres dbname=storage_test sslmode=disable" \
go test ./app/service/... -run TestMigrate -count=1
```

**会清空 DSN 指向的库**（DROP SCHEMA public），只能指向专用测试库；不设置时这两个测试跳过。

## 4. 维护约定

1. **新增页面/入口**：在 `tests/e2e` 对应 spec 增加「可打开 / 关键操作」断言。
2. **选择器**：UI 优先用可见文案/角色（`getByText`/`getByRole`/`getByPlaceholder`），避免绑定内部 class；文案变更时同步更新测试。
3. **环境隔离**：UI 测试只依赖 URL 环境变量；不要写死在业务代码里。
