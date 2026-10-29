// Command fix-ts-field-names 把 TS 生成物中的字段名从 lowerCamelCase 改回 proto 原名（snake_case）。
//
// HTTP 层（pkg/codecx）用 protojson UseProtoNames 输出 proto 原名，但 protoc-gen-typescript-http
// 写死用 field.JSONName()（lowerCamelCase）生成消息类型和 query 参数，且不支持插件参数。
// 本工具读取 buf 的 FileDescriptorSet，按消息精确改写：
//   - 消息类型的字段键：actorNickname → actor_nickname
//   - 客户端方法里读请求字段与拼 query 的地方：request.lastId → request.last_id、`lastId=` → `last_id=`
//
// 须在 fix-ts-int64 之后执行（它按 lowerCamelCase 名匹配字段）。重复执行无副作用。
//
// 用法（make api 已集成）：
//
//	go run ./scripts/proto/fix-ts-field-names -dir api/gen/ts
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/descriptorpb"
)

// msgInfo 需要改名的消息（path 为嵌套消息用下划线连接的名字；renames 为 jsonName → proto 名）
type msgInfo struct {
	pkg     string
	path    string
	renames map[string]string
}

func main() {
	dir := flag.String("dir", "api/gen/ts", "TS 生成目录")
	descriptor := flag.String("descriptor", "", "FileDescriptorSet JSON（默认调用 buf build 生成）")
	flag.Parse()

	fds, cleanup, err := loadDescriptor(*descriptor)
	if err != nil {
		fail(err)
	}
	defer cleanup()

	msgs, global, err := collectMessages(fds)
	if err != nil {
		fail(err)
	}
	changed, err := processDir(*dir, msgs, global)
	if err != nil {
		fail(err)
	}
	fmt.Printf("fix-ts-field-names: %d 个消息含驼峰字段，%d 个文件已更新\n", len(msgs), changed)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "fix-ts-field-names: %v\n", err)
	os.Exit(1)
}

// loadDescriptor 读取 FileDescriptorSet；未指定文件时调用 buf build 生成临时文件
func loadDescriptor(path string) (*descriptorpb.FileDescriptorSet, func(), error) {
	cleanup := func() {}
	if path == "" {
		f, err := os.CreateTemp("", "proto-descriptor-*.json")
		if err != nil {
			return nil, cleanup, err
		}
		name := f.Name()
		_ = f.Close()
		cleanup = func() { _ = os.Remove(name) }

		cmd := exec.Command("buf", "build", "-o", name, "--as-file-descriptor-set")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return nil, cleanup, fmt.Errorf("buf build: %w", err)
		}
		path = name
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, cleanup, err
	}
	fds := &descriptorpb.FileDescriptorSet{}
	// descriptor 里含 buf.validate 等自定义 option，本工具只关心字段名，忽略未知字段
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(raw, fds); err != nil {
		return nil, cleanup, fmt.Errorf("parse descriptor %s: %w", path, err)
	}
	return fds, cleanup, nil
}

// collectMessages 收集 jsonName 与 proto 名不同的字段（跳过 map entry 这类合成消息）。
// global 是全部消息合并后的 jsonName → proto 名，用于客户端方法里的 request.xxx 与 query 键；
// 同一个 jsonName 对应多个 proto 名时无法安全改写，直接报错。
func collectMessages(fds *descriptorpb.FileDescriptorSet) ([]msgInfo, map[string]string, error) {
	var out []msgInfo
	global := map[string]string{}
	var walk func(pkg, under string, msgs []*descriptorpb.DescriptorProto) error
	walk = func(pkg, under string, msgs []*descriptorpb.DescriptorProto) error {
		for _, m := range msgs {
			if m.GetOptions().GetMapEntry() {
				continue
			}
			u := m.GetName()
			if under != "" {
				u = under + "_" + u
			}
			mi := msgInfo{pkg: pkg, path: u, renames: map[string]string{}}
			for _, f := range m.GetField() {
				jsonName, name := f.GetJsonName(), f.GetName()
				if jsonName == "" || jsonName == name {
					continue
				}
				mi.renames[jsonName] = name
				if prev, ok := global[jsonName]; ok && prev != name {
					return fmt.Errorf("jsonName %q 同时对应 proto 字段 %q 与 %q，无法安全改写", jsonName, prev, name)
				}
				global[jsonName] = name
			}
			if len(mi.renames) > 0 {
				out = append(out, mi)
			}
			if err := walk(pkg, u, m.GetNestedType()); err != nil {
				return err
			}
		}
		return nil
	}
	for _, f := range fds.GetFile() {
		if err := walk(f.GetPackage(), "", f.GetMessageType()); err != nil {
			return nil, nil, err
		}
	}
	return out, global, nil
}

// processDir 遍历生成目录，按文件所属 proto package 解析 TS 类型名并改写
func processDir(dir string, msgs []msgInfo, global map[string]string) (int, error) {
	changed := 0
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".ts") {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		pkg := strings.ReplaceAll(filepath.ToSlash(filepath.Dir(rel)), "/", ".")
		ok, err := processFile(path, pkg, msgs, global)
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

var (
	// 消息类型里的字段行：`  actorNickname: string | undefined;` / `  avatar?: Foo;`
	fieldLineRe = regexp.MustCompile(`^(\s+)([A-Za-z_][A-Za-z0-9_]*)(\??: )`)
	// 客户端方法里读请求字段：request.lastId / request?.lastId / request.filter.petId
	requestRefRe = regexp.MustCompile(`(request\??\.)([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)`)
	// 拼 query 的键：queryParams.push(`lastId=...`) / queryParams.push(`labels[key]=...`)
	queryKeyRe = regexp.MustCompile("(queryParams\\.push\\(`)([A-Za-z_][A-Za-z0-9_.]*)")
)

func processFile(path, pkg string, msgs []msgInfo, global map[string]string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	content := string(raw)

	// 生成器把本包内的 message 用裸名输出，跨包时加 package 前缀（点号去掉 + "_"）
	byName := make(map[string]map[string]string, len(msgs))
	for _, m := range msgs {
		name := m.path
		if m.pkg != pkg {
			name = packagePrefix(m.pkg) + m.path
		}
		byName[name] = m.renames
	}

	lines := strings.Split(content, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "export type ") && strings.HasSuffix(line, " = {") {
			name := strings.TrimSuffix(strings.TrimPrefix(line, "export type "), " = {")
			end := -1
			for j := i + 1; j < len(lines); j++ {
				if lines[j] == "};" {
					end = j
					break
				}
			}
			if end < 0 {
				return false, fmt.Errorf("%s: type %s 缺少结束符", path, name)
			}
			if renames, ok := byName[name]; ok {
				found := map[string]bool{}
				for j := i + 1; j < end; j++ {
					lines[j] = fieldLineRe.ReplaceAllStringFunc(lines[j], func(s string) string {
						sub := fieldLineRe.FindStringSubmatch(s)
						proto, ok := renames[sub[2]]
						if !ok {
							return s
						}
						found[sub[2]] = true
						return sub[1] + proto + sub[3]
					})
				}
				for jsonName := range renames {
					if !found[jsonName] && !alreadyRenamed(lines[i+1:end], renames[jsonName]) {
						return false, fmt.Errorf("%s: type %s 中未找到字段 %q（生成器输出格式可能已变化）", path, name, jsonName)
					}
				}
			}
			i = end
			continue
		}
		line = requestRefRe.ReplaceAllStringFunc(line, func(s string) string {
			sub := requestRefRe.FindStringSubmatch(s)
			return sub[1] + renamePath(sub[2], global)
		})
		line = queryKeyRe.ReplaceAllStringFunc(line, func(s string) string {
			sub := queryKeyRe.FindStringSubmatch(s)
			return sub[1] + renamePath(sub[2], global)
		})
		lines[i] = line
	}

	updated := strings.Join(lines, "\n")
	if updated == content {
		return false, nil
	}
	return true, os.WriteFile(path, []byte(updated), 0o644)
}

// renamePath 逐段改写 a.b.c 形式的字段路径；不认识的段（如 toString）原样保留
func renamePath(p string, global map[string]string) string {
	segs := strings.Split(p, ".")
	for i, s := range segs {
		if proto, ok := global[s]; ok {
			segs[i] = proto
		}
	}
	return strings.Join(segs, ".")
}

// alreadyRenamed 字段已是 proto 名（重复执行本工具时）
func alreadyRenamed(lines []string, proto string) bool {
	for _, l := range lines {
		if sub := fieldLineRe.FindStringSubmatch(l); sub != nil && sub[2] == proto {
			return true
		}
	}
	return false
}

// packagePrefix 与 protoc-gen-typescript-http 的命名规则保持一致（service.hello_service → servicehello_service_）
func packagePrefix(pkg string) string {
	return strings.Join(strings.Split(pkg, "."), "") + "_"
}
