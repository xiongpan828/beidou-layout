// Package dbxtest 迁移测试辅助：空库跑迁移、校验 gorm 模型与迁移建出的表结构一致。
package dbxtest

import (
	"context"
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/xiongpan828/beidou-layout/pkg/dbx"
)

// Fresh 连环境变量 envKey 指向的测试库并清空 public schema；没设置则跳过。
// 库里原有的表和数据会全部删除，只能指向专用测试库。
func Fresh(t *testing.T, envKey string) *gorm.DB {
	t.Helper()
	dsn := os.Getenv(envKey)
	if dsn == "" {
		t.Skip(envKey + " not set")
	}
	db, cleanup, err := dbx.NewGorm(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	if err := db.Exec(`DROP SCHEMA public CASCADE; CREATE SCHEMA public`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

var ddl = regexp.MustCompile(`(?i)^\s*(ALTER|CREATE|DROP|COMMENT)\b`)

var errRollback = errors.New("rollback")

// ModelsMatchSchema 迁移后的表结构必须与 gorm 模型一致：对模型再跑一次 AutoMigrate（事务内，结束回滚），
// gorm 认为需要的任何 DDL（加列、改类型 / 长度 / 默认值、建表、建索引）都说明模型和迁移对不上。
// 部分索引、表达式索引只在迁移里建（模型不声明），不在比较范围内。
func ModelsMatchSchema(t *testing.T, db *gorm.DB, models map[any]any) {
	t.Helper()
	rec := &recorder{}
	err := db.Session(&gorm.Session{Logger: rec}).Transaction(func(tx *gorm.DB) error {
		for m := range models {
			if err := tx.AutoMigrate(m); err != nil {
				return err
			}
		}
		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatal(err)
	}
	for _, s := range rec.ddl {
		if same(db, s) {
			continue
		}
		t.Errorf("model differs from migrated schema, gorm wants: %s", s)
	}
}

var (
	alterType    = regexp.MustCompile(`^ALTER TABLE "(\w+)" ALTER COLUMN "(\w+)" TYPE (\w+)(?:\((\d+)\))?`)
	alterDefault = regexp.MustCompile(`^ALTER TABLE "(\w+)" ALTER COLUMN "(\w+)" SET DEFAULT (.+)$`)
	castSuffix   = regexp.MustCompile(`::[\w ]+(\[\])?$`)
	typeAlias    = map[string]string{"varchar": "character varying", "int": "integer", "int4": "integer", "int8": "bigint", "bool": "boolean"}
)

// same gorm 比较默认值 / 类型时不认 PostgreSQL 的类型转换写法（'{}' 与 '{}'::jsonb、null 与 NULL::varchar），
// 会生成语义上什么都不改的 ALTER；对照 information_schema 里的实际定义，相同的不算差异。
func same(db *gorm.DB, stmt string) bool {
	var col struct {
		DataType      string
		CharMaxLength *int
		ColumnDefault *string
	}
	find := func(table, column string) bool {
		return db.Raw(`SELECT data_type, character_maximum_length AS char_max_length, column_default FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`, table, column).Scan(&col).Error == nil && col.DataType != ""
	}
	if m := alterType.FindStringSubmatch(stmt); m != nil && find(m[1], m[2]) {
		want := m[3]
		if a, ok := typeAlias[want]; ok {
			want = a
		}
		return want == col.DataType && (m[4] == "" || col.CharMaxLength != nil && strconv.Itoa(*col.CharMaxLength) == m[4])
	}
	if m := alterDefault.FindStringSubmatch(stmt); m != nil && find(m[1], m[2]) {
		norm := func(v string) string {
			return strings.ToLower(strings.Trim(castSuffix.ReplaceAllString(strings.TrimSpace(v), ""), "'"))
		}
		have := "null"
		if col.ColumnDefault != nil {
			have = norm(*col.ColumnDefault)
		}
		return norm(m[3]) == have
	}
	return false
}

// recorder 只记录 DDL 的 gorm logger
type recorder struct{ ddl []string }

func (r *recorder) LogMode(logger.LogLevel) logger.Interface { return r }
func (r *recorder) Info(context.Context, string, ...any)     {}
func (r *recorder) Warn(context.Context, string, ...any)     {}
func (r *recorder) Error(context.Context, string, ...any)    {}
func (r *recorder) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	if s, _ := fc(); ddl.MatchString(s) {
		r.ddl = append(r.ddl, s)
	}
}
