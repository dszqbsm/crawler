package sqldb

// 函数式选项模式

import (
	"go.uber.org/zap"
)

type options struct {
	logger *zap.Logger
	sqlUrl string
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
func WithConnUrl(sqlUrl string) Option {
	return func(opts *options) {
		opts.sqlUrl = sqlUrl
	}
}
