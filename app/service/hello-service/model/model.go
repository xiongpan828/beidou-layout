// Package model gorm 模型与业务接口
package model

// models gorm gen 模型注册表：key = 模型，value = DIY 查询接口（gen 写不出来的 SQL；没有就写 func() {}）
var models = map[any]any{
	&Greeting{}: func() {},
}

// GetModels gorm gen 模型注册表
func GetModels() map[any]any { return models }
