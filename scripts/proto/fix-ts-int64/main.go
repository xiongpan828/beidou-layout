// Command fix-ts-int64 修正 TS 生成物中 64 位整数的类型。
//
// proto3 JSON 规范把 int64/uint64/sint64/fixed64/sfixed64 编码为字符串，
// 但 protoc-gen-typescript-http（v0.10.0）把它们统一生成为 number，
// 导致 TS 类型与实际响应不一致（见 docs/decisions.md #22）。
// 本工具读取 buf 的 FileDescriptorSet，把对应字段的 TS 类型改成 string；
// 前端在使用处用 Number() 转换（JS 安全整数 2^53，文件大小等场景足够）。
//
// 用法（make api 已集成）：
//
//	go run ./scripts/proto/fix-ts-int64 -dir api/gen/ts
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

type fieldKind int

const (
	kindScalar fieldKind = iota // int64 total
	kindList                    // repeated int64 totals
	kindMap                     // map<string, int64>
)

type fieldInfo struct {
	jsonName string
	kind     fieldKind
}

// msgInfo 64 位字段所在的消息（path 为嵌套消息用下划线连接的名字）
type msgInfo struct {
	pkg    string
	path   string
	fields []fieldInfo
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

	msgs := collectMessages(fds)
	changed, err := processDir(*dir, msgs)
	if err != nil {
		fail(err)
	}
	fmt.Printf("fix-ts-int64: %d 个 64 位字段消息，%d 个文件已更新\n", len(msgs), changed)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "fix-ts-int64: %v\n", err)
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
	// descriptor 里含 buf.validate 等自定义 option，本工具只关心字段类型/名字，忽略未知字段
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(raw, fds); err != nil {
		return nil, cleanup, fmt.Errorf("parse descriptor %s: %w", path, err)
	}
	return fds, cleanup, nil
}

// collectMessages 收集含 64 位字段的消息（跳过 map entry 这类合成消息）
func collectMessages(fds *descriptorpb.FileDescriptorSet) []msgInfo {
	mapValue64 := map[string]bool{}
	var scanMaps func(prefix string, msgs []*descriptorpb.DescriptorProto)
	scanMaps = func(prefix string, msgs []*descriptorpb.DescriptorProto) {
		for _, m := range msgs {
			full := prefix + "." + m.GetName()
			if m.GetOptions().GetMapEntry() {
				for _, f := range m.GetField() {
					if f.GetNumber() == 2 && is64(f.GetType()) {
						mapValue64[full] = true
					}
				}
				continue
			}
			scanMaps(full, m.GetNestedType())
		}
	}
	for _, f := range fds.GetFile() {
		scanMaps("."+f.GetPackage(), f.GetMessageType())
	}

	var out []msgInfo
	var walk func(pkg, dotted, under string, msgs []*descriptorpb.DescriptorProto)
	walk = func(pkg, dotted, under string, msgs []*descriptorpb.DescriptorProto) {
		for _, m := range msgs {
			if m.GetOptions().GetMapEntry() {
				continue
			}
			name := m.GetName()
			d, u := name, name
			if dotted != "" {
				d = dotted + "." + name
				u = under + "_" + name
			}
			mi := msgInfo{pkg: pkg, path: u}
			for _, f := range m.GetField() {
				isMap := f.GetType() == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE && mapValue64[f.GetTypeName()]
				if !is64(f.GetType()) && !isMap {
					continue
				}
				kind := kindScalar
				if f.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
					if isMap {
						kind = kindMap
					} else {
						kind = kindList
					}
				}
				mi.fields = append(mi.fields, fieldInfo{jsonName: f.GetJsonName(), kind: kind})
			}
			if len(mi.fields) > 0 {
				out = append(out, mi)
			}
			walk(pkg, d, u, m.GetNestedType())
		}
	}
	for _, f := range fds.GetFile() {
		walk(f.GetPackage(), "", "", f.GetMessageType())
	}
	return out
}

func is64(t descriptorpb.FieldDescriptorProto_Type) bool {
	switch t {
	case descriptorpb.FieldDescriptorProto_TYPE_INT64,
		descriptorpb.FieldDescriptorProto_TYPE_UINT64,
		descriptorpb.FieldDescriptorProto_TYPE_SINT64,
		descriptorpb.FieldDescriptorProto_TYPE_FIXED64,
		descriptorpb.FieldDescriptorProto_TYPE_SFIXED64:
		return true
	}
	return false
}

// processDir 遍历生成目录，按文件所属 proto package 解析 TS 类型名并改写
func processDir(dir string, msgs []msgInfo) (int, error) {
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
		ok, err := processFile(path, pkg, msgs)
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

func processFile(path, pkg string, msgs []msgInfo) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	content := string(raw)

	// 生成器把本包内的 message 用裸名输出，跨包时加 package 前缀（点号去掉 + "_"）
	byName := make(map[string][]fieldInfo, len(msgs))
	for _, m := range msgs {
		name := m.path
		if m.pkg != pkg {
			name = packagePrefix(m.pkg) + m.path
		}
		byName[name] = m.fields
	}

	lines := strings.Split(content, "\n")
	for i := 0; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "export type ") || !strings.HasSuffix(lines[i], " = {") {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(lines[i], "export type "), " = {")
		fields, ok := byName[name]
		if !ok {
			continue
		}
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
		for _, f := range fields {
			re, repl := fieldReplacer(f)
			found := false
			for j := i + 1; j < end; j++ {
				if re.MatchString(lines[j]) {
					lines[j] = re.ReplaceAllString(lines[j], repl)
					found = true
				}
			}
			if !found {
				return false, fmt.Errorf("%s: type %s 中未找到 64 位字段 %q（生成器输出格式可能已变化）", path, name, f.jsonName)
			}
		}
		i = end
	}

	updated := strings.Join(lines, "\n")
	// google.protobuf.Int64Value / UInt64Value 也是字符串编码，改类型别名即可
	updated = strings.ReplaceAll(updated, "type wellKnownInt64Value = number | null;", "type wellKnownInt64Value = string | null;")
	updated = strings.ReplaceAll(updated, "type wellKnownUInt64Value = number | null;", "type wellKnownUInt64Value = string | null;")

	if updated == content {
		return false, nil
	}
	return true, os.WriteFile(path, []byte(updated), 0o644)
}

// fieldReplacer 返回匹配「字段行」的正则与替换模板
func fieldReplacer(f fieldInfo) (*regexp.Regexp, string) {
	name := regexp.QuoteMeta(f.jsonName)
	switch f.kind {
	case kindList:
		return regexp.MustCompile(`^(\s*)` + name + `: number\[\] \| undefined;$`), `${1}` + f.jsonName + `: string[] | undefined;`
	case kindMap:
		return regexp.MustCompile(`^(\s*)` + name + `: \{ \[key: string\]: number \} \| undefined;$`), `${1}` + f.jsonName + `: { [key: string]: string } | undefined;`
	default:
		return regexp.MustCompile(`^(\s*)` + name + `: number \| undefined;$`), `${1}` + f.jsonName + `: string | undefined;`
	}
}

// packagePrefix 与 protoc-gen-typescript-http 的命名规则保持一致（service.hello_service → servicehello_service_）
func packagePrefix(pkg string) string {
	return strings.Join(strings.Split(pkg, "."), "") + "_"
}
