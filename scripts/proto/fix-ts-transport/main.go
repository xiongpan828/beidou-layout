// Command fix-ts-transport 移除 kratos 版 TS 插件在每个生成文件里重复导出的传输层基础设施
// （ClientTransport / ServerStream / DuplexStream / TransportOptions / SSETransport /
// WSTransport / createDefaultTransport）。
//
// 这些类型/类在每个 service 生成文件中完全相同，前端统一由 app/web/admin/src/lib/http.ts 注入自定义
// transport，不会用到生成器自带的 createDefaultTransport/EventSource 实现。
// 保留 export 会让 `export type * from '@beidou-layout/api-gen/...'` 在多模块间产生 TS2308 名称歧义，
// 因此这里只把它们降级为「文件内私有」（去掉 export），消息类型仍正常导出。
//
// 用法（make api 已集成）：
//
//	go run ./scripts/proto/fix-ts-transport -dir api/gen/ts
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 与 internal/plugin/transportgen.go 生成的声明一一对应（整行匹配，避免误伤引用处）
var exportedDecls = []string{
	"export interface ClientTransport {",
	"export interface ServerStream<T> {",
	"export interface DuplexStream<TIn, TOut> extends ServerStream<TOut> {",
	"export interface TransportOptions {",
	"export class SSETransport<T> implements ServerStream<T> {",
	"export class WSTransport<TIn, TOut> implements DuplexStream<TIn, TOut> {",
	"export function createDefaultTransport(opts?: TransportOptions): ClientTransport {",
}

func main() {
	dir := flag.String("dir", "api/gen/ts", "TS 生成目录")
	flag.Parse()

	changed, err := processDir(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fix-ts-transport: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("fix-ts-transport: %d 个文件已更新\n", changed)
}

func processDir(dir string) (int, error) {
	changed := 0
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".ts") {
			return nil
		}
		ok, err := processFile(path)
		if err != nil {
			return err
		}
		if ok {
			changed++
		}
		return nil
	})
	return changed, err
}

func processFile(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	content := string(raw)
	updated := content
	for _, decl := range exportedDecls {
		updated = strings.ReplaceAll(updated, decl, strings.TrimPrefix(decl, "export "))
	}
	if updated == content {
		return false, nil
	}
	return true, os.WriteFile(path, []byte(updated), 0o644)
}
