package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func testField(name, jsonName string) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{Name: proto.String(name), JsonName: proto.String(jsonName)}
}

func testDescriptor() *descriptorpb.FileDescriptorSet {
	return &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{
		{
			Package: proto.String("service.demo_service"),
			MessageType: []*descriptorpb.DescriptorProto{{
				Name:  proto.String("Item"),
				Field: []*descriptorpb.FieldDescriptorProto{testField("id", "id"), testField("goal_weight_kg", "goalWeightKg")},
				NestedType: []*descriptorpb.DescriptorProto{{
					Name:  proto.String("Tag"),
					Field: []*descriptorpb.FieldDescriptorProto{testField("tag_name", "tagName")},
				}},
			}},
		},
		{
			Package: proto.String("api.push"),
			MessageType: []*descriptorpb.DescriptorProto{{
				Name:  proto.String("WatchReq"),
				Field: []*descriptorpb.FieldDescriptorProto{testField("last_id", "lastId")},
			}},
		},
	}}
}

// TestCollectMessages 只收集 jsonName 与 proto 名不同的字段，嵌套消息用下划线连接
func TestCollectMessages(t *testing.T) {
	msgs, global, err := collectMessages(testDescriptor())
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 {
		t.Fatalf("collectMessages() = %d messages, want 3: %+v", len(msgs), msgs)
	}
	if msgs[0].path != "Item" || msgs[0].renames["goalWeightKg"] != "goal_weight_kg" || len(msgs[0].renames) != 1 {
		t.Errorf("Item = %+v", msgs[0])
	}
	if msgs[1].path != "Item_Tag" || msgs[1].renames["tagName"] != "tag_name" {
		t.Errorf("Item_Tag = %+v", msgs[1])
	}
	if global["lastId"] != "last_id" {
		t.Errorf("global = %+v", global)
	}
}

// TestCollectMessagesConflict 同一个 jsonName 对应不同 proto 名时报错
func TestCollectMessagesConflict(t *testing.T) {
	fds := testDescriptor()
	fds.File[1].MessageType[0].Field = append(fds.File[1].MessageType[0].Field, testField("goalWeightKg", "goalWeightKg"), testField("goal_weight_k_g", "goalWeightKg"))
	if _, _, err := collectMessages(fds); err == nil {
		t.Fatal("期望 jsonName 冲突报错")
	}
}

// TestProcessFile 校验类型字段（含跨包前缀、可选字段）、request 访问与 query 键改写，且重复执行无变化
func TestProcessFile(t *testing.T) {
	msgs, global, err := collectMessages(testDescriptor())
	if err != nil {
		t.Fatal(err)
	}
	src := strings.Join([]string{
		"export type WatchReq = {",
		"  lastId: number | undefined;",
		"};",
		"",
		"export type servicedemo_service_Item = {",
		"  // 目标体重",
		"  goalWeightKg?: number;",
		"  id: number | undefined;",
		"};",
		"",
		"export type servicedemo_service_Item_Tag = {",
		"  tagName: string | undefined;",
		"};",
		"",
		"      if (request.lastId) {",
		"        queryParams.push(`lastId=${encodeURIComponent(request.lastId.toString())}`)",
		"      }",
		"",
	}, "\n")
	want := strings.Join([]string{
		"export type WatchReq = {",
		"  last_id: number | undefined;",
		"};",
		"",
		"export type servicedemo_service_Item = {",
		"  // 目标体重",
		"  goal_weight_kg?: number;",
		"  id: number | undefined;",
		"};",
		"",
		"export type servicedemo_service_Item_Tag = {",
		"  tag_name: string | undefined;",
		"};",
		"",
		"      if (request.last_id) {",
		"        queryParams.push(`last_id=${encodeURIComponent(request.last_id.toString())}`)",
		"      }",
		"",
	}, "\n")

	path := filepath.Join(t.TempDir(), "index.ts")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	for round := 1; round <= 2; round++ {
		changed, err := processFile(path, "api.push", msgs, global)
		if err != nil {
			t.Fatal(err)
		}
		if changed != (round == 1) {
			t.Fatalf("round %d changed = %v", round, changed)
		}
		got, _ := os.ReadFile(path)
		if string(got) != want {
			t.Fatalf("round %d:\n%s\nwant:\n%s", round, got, want)
		}
	}
}

// TestProcessFileMissingField 生成器格式变化导致找不到字段时报错，而不是静默跳过
func TestProcessFileMissingField(t *testing.T) {
	msgs, global, err := collectMessages(testDescriptor())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "index.ts")
	if err := os.WriteFile(path, []byte("export type WatchReq = {\n  \"lastId\": number;\n};\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := processFile(path, "api.push", msgs, global); err == nil {
		t.Fatal("期望找不到字段时报错")
	}
}
