package engine

import (
	"github.com/dszqbsm/crawler/collect"
	"go.uber.org/zap"
)

type Option func(opts *options)

// 爬虫配置选项
type options struct {
	WorkCount int             // 工作线程数，用于控制并发量
	Fetcher   collect.Fetcher // 采集器
	Logger    *zap.Logger     // 日志
	Seeds     []*collect.Task // 初始种子任务
	scheduler Scheduler       // 调度器
}

var defaultOptions = options{
	Logger: zap.NewNop(),
}

func WithLogger(logger *zap.Logger) Option {
	return func(opts *options) {
		opts.Logger = logger
	}
}
func WithFetcher(fetcher collect.Fetcher) Option {
	return func(opts *options) {
		opts.Fetcher = fetcher
	}
}

func WithWorkCount(workCount int) Option {
	return func(opts *options) {
		opts.WorkCount = workCount
	}
}

func WithSeeds(seed []*collect.Task) Option {
	return func(opts *options) {
		opts.Seeds = seed
	}
}

func WithScheduler(scheduler Scheduler) Option {
	return func(opts *options) {
		opts.scheduler = scheduler
	}
}
