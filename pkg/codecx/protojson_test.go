package codecx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v3/encoding"
	"github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// FieldDescriptorProto 自带 type_name / json_name 等下划线字段，用它验证字段名风格，免得 pkg 依赖业务 proto
func sample() *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		TypeName: proto.String(".demo.Greeting"),
		JsonName: proto.String("petName"),
	}
}

func TestResponseEncoderUsesProtoNames(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	if err := responseEncoder(w, r, sample()); err != nil {
		t.Fatal(err)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"type_name"`) || strings.Contains(body, `"typeName"`) {
		t.Fatalf("响应应输出 proto 原名：%s", body)
	}
}

func TestStreamCodecUsesProtoNames(t *testing.T) {
	codec := encoding.GetCodec("protojson")
	if codec == nil {
		t.Fatal("kratos protojson codec 未注册")
	}
	data, err := codec.Marshal(sample())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"type_name"`) || strings.Contains(string(data), `"typeName"`) {
		t.Fatalf("流式接口应输出 proto 原名：%s", data)
	}
}

func TestRequestDecoderAcceptsBothNames(t *testing.T) {
	for _, body := range []string{`{"type_name":".demo.Greeting"}`, `{"typeName":".demo.Greeting"}`} {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		got := &descriptorpb.FieldDescriptorProto{}
		if err := requestDecoder(r, got); err != nil {
			t.Fatalf("%s: %v", body, err)
		}
		if got.GetTypeName() != ".demo.Greeting" {
			t.Fatalf("%s: type_name=%q", body, got.GetTypeName())
		}
	}
}

func decodeBody(t *testing.T, body string, msg proto.Message) error {
	t.Helper()
	return requestDecoder(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)), msg)
}

func TestRequestDecoderToleratesUnknownFields(t *testing.T) {
	got := &descriptorpb.FieldDescriptorProto{}
	if err := decodeBody(t, `{"type_name":".demo.Greeting","type":"TYPE_STRING","from_new_client":1}`, got); err != nil {
		t.Fatal(err)
	}
	if got.GetTypeName() != ".demo.Greeting" || got.GetType() != descriptorpb.FieldDescriptorProto_TYPE_STRING {
		t.Fatalf("got %v", got)
	}
}

// 宽松解码会把认不出的枚举名静默当成 0；拼错的名字、改名前的旧名都要报错，不能悄悄落到默认值
func TestRequestDecoderRejectsUnknownEnumNames(t *testing.T) {
	for name, body := range map[string]string{
		"顶层":          `{"type":"TYPE_STRNG"}`,
		"顶层 + 未知字段":   `{"from_new_client":1,"type":"TYPE_STRNG"}`,
		"嵌套 repeated": `{"name":"Greeting","field":[{"name":"a","label":"LABEL_SOMETIMES"}],"from_new_client":1}`,
	} {
		msg := proto.Message(&descriptorpb.FieldDescriptorProto{})
		if strings.HasPrefix(name, "嵌套") {
			msg = &descriptorpb.DescriptorProto{}
		}
		err := decodeBody(t, body, msg)
		if e := errors.FromError(err); e == nil || e.Code != http.StatusBadRequest || e.Reason != "VALIDATOR" || !strings.Contains(e.Message, "invalid value for enum field") {
			t.Errorf("%s: want 400 VALIDATOR enum error, got %v", name, err)
		}
	}
}

func TestRequestDecoderAcceptsEnumNumbers(t *testing.T) {
	got := &descriptorpb.FieldDescriptorProto{}
	if err := decodeBody(t, `{"type":9,"from_new_client":1}`, got); err != nil || got.GetType() != descriptorpb.FieldDescriptorProto_TYPE_STRING {
		t.Fatalf("err=%v got=%v", err, got)
	}
}

func TestRequestDecoderMalformedJSONIsCodecError(t *testing.T) {
	err := decodeBody(t, `{"type":`, &descriptorpb.FieldDescriptorProto{})
	if e := errors.FromError(err); e == nil || e.Reason != "CODEC" {
		t.Fatalf("want CODEC, got %v", err)
	}
}
