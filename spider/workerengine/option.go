package workerengine

import (
	"github.com/dszqbsm/crawler/spider"
	"go.uber.org/zap"
)

type Option func(opts *options)

// 爬虫配置选项
type options struct {
	id                 string
	WorkCount          int            // 工作线程数，用于控制并发量
	Fetcher            spider.Fetcher // 采集器
	Storage            spider.DataRepository
	Logger             *zap.Logger    // 日志
	Seeds              []*spider.Task // 初始种子任务
	registryURL        string
	scheduler          Scheduler // 调度器
	reqRepository      spider.ReqHistoryRepository
	resourceRepository spider.ResourceRepository
	resourceRegistry   spider.ResourceRegistry
}

var defaultOptions = options{
	Logger: zap.NewNop(),
}

func WithID(id string) Option {
	return func(opts *options) {
		opts.id = id
	}
}

func WithReqRepository(reqRepository spider.ReqHistoryRepository) Option {
	return func(opts *options) {
		opts.reqRepository = reqRepository
	}
}

func WithResourceRepository(resourceRepository spider.ResourceRepository) Option {
	return func(opts *options) {
		opts.resourceRepository = resourceRepository
	}
}

func WithResourceRegistry(resourceRegistry spider.ResourceRegistry) Option {
	return func(opts *options) {
		opts.resourceRegistry = resourceRegistry
	}
}

func WithStorage(s spider.DataRepository) Option {
	return func(opts *options) {
		opts.Storage = s
	}
}

func WithLogger(logger *zap.Logger) Option {
	return func(opts *options) {
		opts.Logger = logger
	}
}
func WithFetcher(fetcher spider.Fetcher) Option {
	return func(opts *options) {
		opts.Fetcher = fetcher
	}
}

func WithWorkCount(workCount int) Option {
	return func(opts *options) {
		opts.WorkCount = workCount
	}
}

func WithSeeds(seed []*spider.Task) Option {
	return func(opts *options) {
		opts.Seeds = seed
	}
}

func WithScheduler(scheduler Scheduler) Option {
	return func(opts *options) {
		opts.scheduler = scheduler
	}
}
