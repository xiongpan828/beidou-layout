// Package codecx 提供 kratos HTTP 的 protojson 编解码选项。
//
// 背景：kratos v3 的 `Accept/Content-Type: application/json` 默认走 `encoding/json`
// （按 Go struct 的 json tag 编码：字段 snake_case，google.protobuf.Timestamp 序列化为
// {seconds,nanos} 对象）。而 proto 生态/生成的 TS 类型约定的是 protojson JSON 映射：
// camelCase 字段名、Timestamp → RFC3339 字符串、enum → 名称、64 位整数 → 字符串。
//
// ProtoJSON 让 proto 消息走 protojson（对齐 grpc-gateway 的行为），同时保持
// Content-Type: application/json；非 proto 值（如 errors.Error / httpbody）回退默认编码。
//
// 字段名输出 proto 原名（snake_case，UseProtoNames），与 HTTP 接口常见风格一致；
// 请求体 / query 解析时 proto 名与 lowerCamelCase 都认。生成的 TS 类型由
// scripts/proto/fix-ts-field-names 改回 proto 名，与这里保持一致。
package codecx

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	kprotojson "github.com/go-kratos/kratos/v3/encoding/protojson"
	"github.com/go-kratos/kratos/v3/errors"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var (
	// 默认不输出零值/未设置字段，与生成 TS 的 `| undefined` 语义一致（同 grpc-gateway 默认）
	marshalOptions = protojson.MarshalOptions{
		EmitUnpopulated: false,
		UseProtoNames:   true,
	}
	// 严格解码：请求里没有多余字段时（常态）一次通过
	strictUnmarshal = protojson.UnmarshalOptions{}
	// 宽松解码：容忍未知字段，便于前后端渐进升级。注意它也会把认不出的枚举名静默当成 0，
	// 所以宽松解码后要再用 checkEnums 把非法枚举值挑出来
	lenientUnmarshal = protojson.UnmarshalOptions{DiscardUnknown: true}
)

// 流式接口（SSE / WebSocket）不经过 ResponseEncoder，走 kratos 注册的 protojson codec，字段名同样用 proto 原名
func init() {
	kprotojson.MarshalOptions.UseProtoNames = true
}

// ProtoJSON 返回让 proto 消息走 protojson 的 HTTP server 选项。
func ProtoJSON() []khttp.ServerOption {
	return []khttp.ServerOption{
		khttp.RequestDecoder(requestDecoder),
		khttp.ResponseEncoder(responseEncoder),
	}
}

// requestDecoder 对 proto 消息用 protojson 解析请求体，其余走 kratos 默认解码。
func requestDecoder(r *http.Request, v any) error {
	msg, ok := v.(proto.Message)
	if !ok {
		return khttp.DefaultRequestDecoder(r, v)
	}
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return errors.BadRequest("CODEC", err.Error())
	}
	r.Body = io.NopCloser(bytes.NewBuffer(data))
	if len(data) == 0 {
		return nil
	}
	if err := unmarshal(data, msg); err != nil {
		// 非法枚举值属于参数校验失败，与 protovalidate 一致返回 VALIDATOR（前端统一提示「请检查输入内容」）
		var ee *enumError
		if stderrors.As(err, &ee) {
			return errors.BadRequest("VALIDATOR", err.Error())
		}
		return errors.BadRequest("CODEC", fmt.Sprintf("body unmarshal %s", err.Error()))
	}
	return nil
}

// enumError 请求里有认不出的枚举名
type enumError struct{ field, value string }

func (e *enumError) Error() string {
	return fmt.Sprintf("invalid value for enum field %s: %q", e.field, e.value)
}

// unmarshal 容忍未知字段，但拒绝非法枚举值（拼错的枚举名、改名前的旧名）
func unmarshal(data []byte, msg proto.Message) error {
	if strictUnmarshal.Unmarshal(data, msg) == nil {
		return nil
	}
	// 严格解码失败可能是未知字段，也可能是非法枚举值：宽松解码后逐个核对枚举字段
	proto.Reset(msg)
	if err := lenientUnmarshal.Unmarshal(data, msg); err != nil {
		return err
	}
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	return checkEnums(msg.ProtoReflect().Descriptor(), raw, "")
}

// specialJSON JSON 形式不是普通对象的知名类型（字符串、数字、任意值），不往里走
var specialJSON = map[protoreflect.FullName]bool{
	"google.protobuf.Any": true, "google.protobuf.Timestamp": true, "google.protobuf.Duration": true,
	"google.protobuf.FieldMask": true, "google.protobuf.Struct": true, "google.protobuf.Value": true,
	"google.protobuf.ListValue": true, "google.protobuf.DoubleValue": true, "google.protobuf.FloatValue": true,
	"google.protobuf.Int64Value": true, "google.protobuf.UInt64Value": true, "google.protobuf.Int32Value": true,
	"google.protobuf.UInt32Value": true, "google.protobuf.BoolValue": true, "google.protobuf.StringValue": true,
	"google.protobuf.BytesValue": true,
}

// checkEnums 对照 message 描述符检查原始 JSON 里的枚举名都认识；未知字段跳过（已被宽松解码丢弃）
func checkEnums(md protoreflect.MessageDescriptor, raw any, path string) error {
	obj, ok := raw.(map[string]any)
	if !ok || specialJSON[md.FullName()] {
		return nil
	}
	fields := md.Fields()
	for key, v := range obj {
		fd := fields.ByJSONName(key)
		if fd == nil {
			fd = fields.ByTextName(key)
		}
		if fd == nil {
			continue
		}
		p := path + "." + key
		var values []any
		switch {
		case fd.IsList():
			values, _ = v.([]any)
		case fd.IsMap():
			m, _ := v.(map[string]any)
			for _, mv := range m {
				values = append(values, mv)
			}
			fd = fd.MapValue()
		default:
			values = []any{v}
		}
		for _, item := range values {
			switch fd.Kind() {
			case protoreflect.EnumKind:
				// 数字形式交给 protovalidate 的 defined_only；这里只管字符串
				if s, ok := item.(string); ok && fd.Enum().Values().ByName(protoreflect.Name(s)) == nil {
					return &enumError{field: strings.TrimPrefix(p, "."), value: s}
				}
			case protoreflect.MessageKind, protoreflect.GroupKind:
				if err := checkEnums(fd.Message(), item, p); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Marshal 与 HTTP 响应相同的 protojson 编码（proto 原名字段、不输出零值），给不走 kratos 路由编码的手写 handler 用
func Marshal(msg proto.Message) ([]byte, error) { return marshalOptions.Marshal(msg) }

// responseEncoder 对 proto 消息用 protojson 编码响应，其余走 kratos 默认编码。
func responseEncoder(w http.ResponseWriter, r *http.Request, v any) error {
	msg, ok := v.(proto.Message)
	if !ok {
		return khttp.DefaultResponseEncoder(w, r, v)
	}
	data, err := marshalOptions.Marshal(msg)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(data)
	return err
}
