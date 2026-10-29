// 错误码分段校验：保证各服务 ErrorReason 落在自己独占的千位段内，从而全局唯一。
// 新增服务的 error.proto 时，在 segments 里登记一行（段号顺延）。
package api_test

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"

	common "github.com/xiongpan828/beidou-layout/api/gen/go/service/common"
	hellosvc "github.com/xiongpan828/beidou-layout/api/gen/go/service/hello_service"
	storagesvc "github.com/xiongpan828/beidou-layout/api/gen/go/service/storage_service"
)

// segments 服务 → 错误码段：段号 N 独占 [N*1000, N*1000+999]，段内百位划分子域（如 21xx 某个领域）
var segments = []struct {
	service string
	segment int32
	enum    protoreflect.EnumDescriptor
}{
	{"common", 1, common.ErrorReason(0).Descriptor()},
	{"hello-service", 2, hellosvc.ErrorReason(0).Descriptor()},
	{"storage-service", 3, storagesvc.ErrorReason(0).Descriptor()},
}

func TestErrorReasonSegments(t *testing.T) {
	owner := map[int32]string{}
	for _, s := range segments {
		if prev, ok := owner[s.segment]; ok {
			t.Fatalf("segment %d assigned to both %s and %s", s.segment, prev, s.service)
		}
		owner[s.segment] = s.service

		lo, hi := s.segment*1000, s.segment*1000+999
		values := s.enum.Values()
		for i := 0; i < values.Len(); i++ {
			v := values.Get(i)
			if n := int32(v.Number()); n != 0 && (n < lo || n > hi) {
				t.Errorf("%s: %s = %d out of segment [%d, %d]", s.service, v.Name(), n, lo, hi)
			}
		}
	}
}
