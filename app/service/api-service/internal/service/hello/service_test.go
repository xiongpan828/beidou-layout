package hello

import (
	"context"
	"testing"

	apihello "github.com/xiongpan828/beidou-layout/api/gen/go/service/api_service/api_hello"
)

type fakeBiz struct{ got string }

func (f *fakeBiz) SayHello(_ context.Context, name string) (string, error) {
	f.got = name
	return "Hello, " + name + "!", nil
}

// service 只做转换：请求里的 name 原样交给 biz，biz 返回的问候语放进响应
func TestSayHello(t *testing.T) {
	biz := &fakeBiz{}
	reply, err := NewHello(biz).SayHello(context.Background(), &apihello.SayHelloReq{Name: "world"})
	if err != nil {
		t.Fatal(err)
	}
	if biz.got != "world" || reply.GetMessage() != "Hello, world!" {
		t.Fatalf("got name %q, message %q", biz.got, reply.GetMessage())
	}
}
