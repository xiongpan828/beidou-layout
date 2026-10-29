package hello

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	hellosvc "github.com/xiongpan828/beidou-layout/api/gen/go/service/hello_service"
	"github.com/xiongpan828/beidou-layout/app/service/hello-service/model"
)

func toProtoGreeting(g *model.Greeting) *hellosvc.Greeting {
	return &hellosvc.Greeting{
		Id:        g.ID,
		Name:      g.Name,
		Message:   g.Message,
		CreatedAt: timestamppb.New(g.CreatedAt),
	}
}
