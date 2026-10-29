// Package validatex 基于 buf protovalidate 的请求校验中间件
package validatex

import (
	"buf.build/go/protovalidate"
	"github.com/go-kratos/kratos/v3/middleware"
	kmw "github.com/go-kratos/kratos/v3/middleware/validate"
	"google.golang.org/protobuf/proto"
)

// New 创建 protovalidate 校验中间件（proto 中 (buf.validate.field) 规则生效）
func New() middleware.Middleware {
	v, err := protovalidate.New()
	if err != nil {
		panic(err)
	}
	return kmw.Validator(func(msg any) error {
		if m, ok := msg.(proto.Message); ok {
			return v.Validate(m)
		}
		return nil
	})
}
