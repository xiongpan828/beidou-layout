package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func testField(name, jsonName string, num int32, typ descriptorpb.FieldDescriptorProto_Type, label descriptorpb.FieldDescriptorProto_Label) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:     proto.String(name),
		JsonName: proto.String(jsonName),
		Number:   proto.Int32(num),
		Type:     typ.Enum(),
		Label:    label.Enum(),
	}
}

// TestCollectMessages 校验 64 位字段识别：标量 / repeated / map value
func TestCollectMessages(t *testing.T) {
	str := descriptorpb.FieldDescriptorProto_TYPE_STRING
	i64 := descriptorpb.FieldDescriptorProto_TYPE_INT64
	u32 := descriptorpb.FieldDescriptorProto_TYPE_UINT32
	msg := descriptorpb.FieldDescriptorProto_TYPE_MESSAGE
	repeated := descriptorpb.FieldDescriptorProto_LABEL_REPEATED
	optional := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL

	mapEntry := &descriptorpb.DescriptorProto{
		Name:    proto.String("LabelsEntry"),
		Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)},
		Field: []*descriptorpb.FieldDescriptorProto{
			testField("key", "key", 1, str, optional),
			testField("value", "value", 2, i64, optional),
		},
	}

	fds := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{{
		Name:    proto.String("p/thing.proto"),
		Package: proto.String("p"),
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("Thing"),
			Field: []*descriptorpb.FieldDescriptorProto{
				testField("total", "total", 1, i64, optional),
				testField("scores", "scores", 2, i64, repeated),
				testField("labels", "labels", 3, msg, repeated),
				testField("page_size", "pageSize", 4, u32, optional),
			},
			NestedType: []*descriptorpb.DescriptorProto{mapEntry},
		}},
	}}}
	fds.File[0].MessageType[0].Field[2].TypeName = proto.String(".p.Thing.LabelsEntry")

	got := collectMessages(fds)
	if len(got) != 1 {
		t.Fatalf("collectMessages() = %d messages, want 1", len(got))
	}
	if got[0].pkg != "p" || got[0].path != "Thing" {
		t.Fatalf("message = %+v, want p/Thing", got[0])
	}
	want := []fieldInfo{
		{jsonName: "total", kind: kindScalar},
		{jsonName: "scores", kind: kindList},
		{jsonName: "labels", kind: kindMap},
	}
	if len(got[0].fields) != len(want) {
		t.Fatalf("fields = %+v, want %+v", got[0].fields, want)
	}
	for i := range want {
		if got[0].fields[i] != want[i] {
			t.Errorf("fields[%d] = %+v, want %+v", i, got[0].fields[i], want[i])
		}
	}
}

// TestProcessFile 校验跨包消息类型名解析与字段改写（含 repeated / map）
func TestProcessFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "index.ts")
	content := `export type servicehello_service_ListGreetingReply = {
  users: string[] | undefined;
  total: number | undefined;
};

export type servicehello_service_Scores = {
  scores: number[] | undefined;
  labels: { [key: string]: number } | undefined;
  pageSize: number | undefined;
};
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	msgs := []msgInfo{
		{pkg: "service.hello_service", path: "ListGreetingReply", fields: []fieldInfo{{jsonName: "total", kind: kindScalar}}},
		{pkg: "service.hello_service", path: "Scores", fields: []fieldInfo{
			{jsonName: "scores", kind: kindList},
			{jsonName: "labels", kind: kindMap},
		}},
	}
	// 文件属于 admin 包，hello_service 的类型需要带包前缀
	if _, err := processFile(path, "service.admin_service.admin_hello", msgs); err != nil {
		t.Fatalf("processFile() error = %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	for _, want := range []string{
		"total: string | undefined;",
		"scores: string[] | undefined;",
		"labels: { [key: string]: string } | undefined;",
		"pageSize: number | undefined;", // uint32 保持 number
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}
