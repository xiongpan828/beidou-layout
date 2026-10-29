// Package enumx 业务枚举与库内字符串互转（库存 proto 枚举名，与 Enum.String() 一致）。
package enumx

import klog "github.com/go-kratos/kratos/v3/log"

// From 库内枚举名 → 枚举值；空或不识别为 0（UNSPECIFIED）。不识别的值多半是漏迁移的旧数据，打警告便于发现。
func From(value map[string]int32, s string) int32 {
	if s == "" {
		return 0
	}
	n, ok := value[s]
	if !ok {
		klog.Warn("unknown enum value, treated as UNSPECIFIED", "value", s)
	}
	return n
}
