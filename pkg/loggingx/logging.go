// Package loggingx kratos logger 初始化（JSON 结构化 + 敏感字段脱敏 + 服务属性）
package loggingx

import (
	klog "github.com/go-kratos/kratos/v3/log"
)

// Init 初始化 kratos logger 并设为全局默认。
// 参考 kratos v3 log README 推荐写法：NewHandler + FilterKey 脱敏 + service 属性。
func Init(serviceName string) {
	logger := klog.NewLogger(
		klog.NewHandler(
			klog.WithFormat(klog.FormatJSON),
			klog.WithLevel(klog.LevelInfo),
			klog.WithAddSource(true),
		),
		// 敏感字段脱敏，防止密码/token 泄露到日志
		klog.WithFilter(klog.FilterKey("password", "secret", "token", "authorization")),
	).With(
		"service.name", serviceName,
	)
	klog.SetDefault(logger)
}
