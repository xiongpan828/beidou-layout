// Package constants 服务自身常量
package constants

// ServiceName 服务名
const ServiceName = "storage-service"

// FilePathPrefix 文件读写的 HTTP 路径前缀：{base_url}/api/v1/storage/file/{key}（网关 /api/v1/storage/* 转发到本服务 HTTP 端口）
const FilePathPrefix = "/api/v1/storage/file/"
