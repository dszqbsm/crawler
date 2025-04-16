package sqlstorage

// 用于配置sql存储相关的选项，用于存储引擎的函数选择模式

import (
	"go.uber.org/zap"
)

type options struct {
	logger     *zap.Logger
	sqlUrl     string
	BatchCount int // 批量数
}

// 默认选项
var defaultOptions = options{
	logger: zap.NewNop(),
}

type Option func(opts *options)

// 配置日志器
func WithLogger(logger *zap.Logger) Option {
	return func(opts *options) {
		opts.logger = logger
	}
}

// 配置数据库的链接url
func WithSqlUrl(sqlUrl string) Option {
	return func(opts *options) {
		opts.sqlUrl = sqlUrl
	}
}

// 配置批量处理的数量
func WithBatchCount(batchCount int) Option {
	return func(opts *options) {
		opts.BatchCount = batchCount
	}
}
