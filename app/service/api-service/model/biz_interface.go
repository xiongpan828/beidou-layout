// Package model 定义 api-service BFF 的业务接口与参数（Service 与 Biz 之间的契约）
package model

import "context"

// IHelloBiz 问候 BFF 业务接口
type IHelloBiz interface {
	// SayHello 问候一次，返回问候语
	SayHello(ctx context.Context, name string) (string, error)
}
