package migrate

import (
	"testing"

	"github.com/xiongpan828/beidou-layout/app/service/hello-service/model"
	"github.com/xiongpan828/beidou-layout/pkg/dbx/dbxtest"
)

// 空库跑全部迁移、再跑一次不报错；迁移建出的表结构与 gorm 模型一致
// （TEST_HELLO_DSN 指向专用测试库，库里原有的表会被清空；没设置则跳过）
func TestMigrate(t *testing.T) {
	db := dbxtest.Fresh(t, "TEST_HELLO_DSN")
	for range 2 {
		if err := Migrate(db); err != nil {
			t.Fatal(err)
		}
	}
	dbxtest.ModelsMatchSchema(t, db, model.GetModels())
}
