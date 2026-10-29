# Third Party Proto

本目录存放 **vendored 的第三方 proto 文件**，所有 proto 依赖都收拢在 `api/` 下：`buf.yaml` 没有远程 deps，生成不依赖 BSR。`google/protobuf/*.proto`（Timestamp、descriptor 等）由 buf 内置提供，不在本目录。

## 依赖清单

| 文件 | 来源 | 用途 | Go 侧对应包（go_package） |
|---|---|---|---|
| `google/api/annotations.proto` | googleapis/googleapis | HTTP 路由注解（`google.api.http`） | `google.golang.org/genproto/googleapis/api/annotations` |
| `google/api/http.proto` | googleapis/googleapis | annotations.proto 的依赖 | 同上 |
| `buf/validate/validate.proto` | bufbuild/protovalidate | 参数校验规则 | `buf.build/gen/go/bufbuild/protovalidate/...` |
| `errors/errors.proto` | go-kratos/kratos | 错误码注解（`errors.code`） | `github.com/go-kratos/kratos/v3/errors` |

## 使用方式

在 proto 中按如下 import（带 `third_party/` 前缀，相对 `api/` 模块）：

```proto
import "third_party/google/api/annotations.proto";
import "third_party/buf/validate/validate.proto";
import "third_party/errors/errors.proto";
```

- `buf.yaml` 的 `lint.ignore` 已忽略本目录，vendored 文件不参与本仓 lint 约束（它们的 package 与目录不一致，过不了 `PACKAGE_DIRECTORY_MATCH`）
- 本目录**不生成代码**：`buf.gen.*.yaml` 的输入只有 `api/service` 与 `api/config`。生成代码引用的是上表 go_package 指向的上游 Go 模块（在 `go.mod` 里），升级 proto 时要保持与 go.mod 中的版本一致

## 升级

需要升级依赖时，从上游仓库（googleapis/googleapis、bufbuild/protovalidate、go-kratos/kratos）复制新版 `.proto` 覆盖对应文件，并同步修改文件内部的 import 前缀为 `third_party/...`（目前只有 `annotations.proto` import 了 `third_party/google/api/http.proto`）；同时升级 `go.mod` 中对应的 Go 模块，然后 `make api` 确认生成物无意外变化。
