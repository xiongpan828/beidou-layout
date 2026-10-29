// Package dao gen 查询封装（支持事务）
package dao

import (
	"context"

	"gorm.io/gorm"
)

// Dao 封装 Query，支持事务与 WithContext
type Dao struct {
	Q *Query
}

// NewDao 初始化
func NewDao(db *gorm.DB) *Dao { return &Dao{Q: Use(db)} }

type txCtxKey struct{}

// WithContext 返回带上下文的 queryCtx 实例；若 ctx 已在事务中则复用事务实例
func (d *Dao) WithContext(ctx context.Context) *queryCtx {
	if q, ok := ctx.Value(txCtxKey{}).(*queryCtx); ok {
		return q
	}
	return d.Q.WithContext(ctx)
}

// Tx 事务：已处于事务中则直接执行，否则开启新事务
func (d *Dao) Tx(ctx context.Context, fn func(context.Context) error) error {
	if _, ok := ctx.Value(txCtxKey{}).(*queryCtx); ok {
		return fn(ctx)
	}
	return d.Q.Transaction(func(tx *Query) error {
		return fn(context.WithValue(ctx, txCtxKey{}, tx.WithContext(ctx)))
	})
}
