package spider

import (
	"sync"
)

type Property struct {
	Name     string `json:"name"`      // 任务名称，应保证唯一性
	URL      string `json:"url"`       // 任务的入口URL
	Cookie   string `json:"cookie"`    // 任务的Cookie
	WaitTime int64  `json:"wait_time"` // 随机休眠时间，秒
	Reload   bool   `json:"reload"`    // 网站是否可以重复爬取
	MaxDepth int64  `json:"max_depth"` // 任务的最大深度
}

type TaskConfig struct {
	Name     string
	Cookie   string
	WaitTime int64
	Reload   bool
	MaxDepth int
	Fetcher  string
	Limits   []LimitCofig
}

type LimitCofig struct {
	EventCount int
	EventDur   int // 秒
	Bucket     int // 桶大小
}

// 一个任务实例，
type Task struct {
	Visited     map[string]bool // 用于记录已访问过的url
	VisitedLock sync.Mutex      // 用于保护Visited
	Rule        RuleTree        // 任务的解析规则
	Options
}

func NewTask(opts ...Option) *Task {
	options := defaultOptions
	for _, opt := range opts {
		opt(&options)
	}

	d := &Task{}
	d.Options = options

	return d
}

type Fetcher interface {
	Get(url *Request) ([]byte, error)
}
