.PHONY: tools api proto-check build build-go run-hello run-storage run-api run-admin migrate gen test test-api test-e2e lint dep-check check-gorm-gen

# 安装 proto 插件到 tools/（版本固定在 scripts/proto/build-tools.sh）
tools:
	./scripts/proto/build-tools.sh

# 生成 proto 代码（Go stub + TS 请求桩 + 配置结构体）
api: tools
	buf generate --template buf.gen.go.yaml
	buf generate --template buf.gen.ts.yaml
	buf generate --template buf.gen.config.yaml
	# 修正 64 位整数（int64/uint64）的 TS 类型：protojson 输出字符串，生成器默认给 number
	go run ./scripts/proto/fix-ts-int64 -dir api/gen/ts
	# 去掉 kratos 版插件在每个文件重复导出的传输层基础设施（避免 export * 名称歧义）
	go run ./scripts/proto/fix-ts-transport -dir api/gen/ts
	# 字段名改回 proto 原名（snake_case），与 HTTP 层 protojson UseProtoNames 一致；须在 fix-ts-int64 之后
	go run ./scripts/proto/fix-ts-field-names -dir api/gen/ts

# proto 校验（lint + 破坏性变更检查，CI 同款）
proto-check:
	buf lint
	buf breaking --against '.git#branch=main'

# 构建 Go 服务
build-go:
	mkdir -p build/bin
	go build -o build/bin/hello-service ./app/service/hello-service/cmd
	go build -o build/bin/storage-service ./app/service/storage-service/cmd
	go build -o build/bin/api-service ./app/service/api-service/cmd/server
	go build -o build/bin/admin-service ./app/service/admin-service/cmd/server

build: build-go

# 本地运行
run-hello:
	go run ./app/service/hello-service/cmd hello -c app/service/hello-service/configs/config.yaml

# 对象存储：gRPC :9300 + 文件读写 HTTP :9301（网关 /api/v1/storage/*）
run-storage:
	go run ./app/service/storage-service/cmd storage -c app/service/storage-service/configs/config.yaml

run-api:
	go run ./app/service/api-service/cmd/server -conf app/service/api-service/configs/config.yaml

run-admin:
	go run ./app/service/admin-service/cmd/server -conf app/service/admin-service/configs/config.yaml

# 数据库迁移（gormigrate）
migrate:
	go run ./app/service/hello-service/cmd migrate -c app/service/hello-service/configs/config.yaml
	go run ./app/service/storage-service/cmd migrate -c app/service/storage-service/configs/config.yaml

# gorm gen 生成 dao（不需要数据库）
gen:
	go run ./app/service/hello-service/cmd gen -c app/service/hello-service/configs/config.yaml
	go run ./app/service/storage-service/cmd gen -c app/service/storage-service/configs/config.yaml

test-api:
	go test -tags=integration ./tests/api -count=1

test-e2e:
	pnpm test:e2e

test:
	go test ./...

# 静态检查：golangci-lint + biz 禁止手写 gorm 条件
lint:
	golangci-lint run ./...
	./scripts/check-gorm-gen.sh

# 弃用 API 检查（staticcheck SA1019）：禁止使用已标记 Deprecated 的标准库/依赖 API
# 安装：go install honnef.co/go/tools/cmd/staticcheck@latest
dep-check:
	staticcheck -checks SA1019 ./...

# biz 禁止手写 gorm 条件（必须走 gen）
check-gorm-gen:
	./scripts/check-gorm-gen.sh
