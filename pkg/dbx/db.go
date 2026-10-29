// Package dbx gorm 初始化（postgres）
package dbx

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// NewGorm 初始化 gorm（postgres + 单数表名推导）。
// 单数表名推导：Greeting → greeting、File → file、Provider → provider（无需每个结构体写 TableName）。
func NewGorm(dsn string) (*gorm.DB, func(), error) {
	return open(dsn, false)
}

// NewGormOffline 不连库的 gorm 实例（启动时不 ping）：gorm gen 只解析模型、不查库，用它生成 dao 不需要起数据库。
func NewGormOffline(dsn string) (*gorm.DB, func(), error) {
	return open(dsn, true)
}

func open(dsn string, offline bool) (*gorm.DB, func(), error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
		DisableAutomaticPing: offline,
	})
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	}
	return db, cleanup, nil
}
