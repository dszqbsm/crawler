package collect

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"math/rand"
	"regexp"
	"sync"
	"time"

	"github.com/dszqbsm/crawler/collector"
	"github.com/dszqbsm/crawler/limiter"
	"go.uber.org/zap"
)

// 爬虫任务
type Task struct {
	Property                    // 任务属性
	Visited     map[string]bool // 用于记录已访问过的url
	VisitedLock sync.Mutex      // 用于保护Visited
	Fetcher     Fetcher         // 负责发起HTTP请求的Fetcher实现
	Storage     collector.Storage
	Rule        RuleTree            // 任务的解析规则
	Logger      *zap.Logger         // 日志器
	Limit       limiter.RateLimiter // 限速器
}

// 爬虫任务的属性
type Property struct {
	Name     string        `json:"name"`      // 任务名称，应保证唯一性
	Url      string        `json:"url"`       // 任务的入口URL
	Cookie   string        `json:"cookie"`    // 任务的Cookie
	WaitTime time.Duration `json:"wait_time"` // 任务的等待时间
	Reload   bool          `json:"reload"`    // 网站是否可以重复爬取
	MaxDepth int           `json:"max_depth"` // 任务的最大深度
}

// 表示解析结果
type ParseResult struct {
	Requests []*Request    // 从当前页面解析出的新请求
	Items    []interface{} // 从当前页面提取的有用数据
}

// 上下文结构体：将响应内容和当前请求封装在一起，传递给解析函数
type Context struct {
	Body []byte   // 响应内容的字节流
	Req  *Request // 当前请求的上下文
}

// 依据规则名称从当前请求任务规则书中获取对应的规则
func (c *Context) GetRule(ruleName string) *Rule {
	return c.Req.Task.Rule.Trunk[ruleName]
}

// 将解析到的data数据封装成一个DataCell对象，并添加一些额外的元信息
func (c *Context) Output(data interface{}) *collector.DataCell {
	res := &collector.DataCell{}
	res.Data = make(map[string]interface{})
	res.Data["Task"] = c.Req.Task.Name
	res.Data["Rule"] = c.Req.RuleName
	res.Data["Data"] = data
	res.Data["Url"] = c.Req.Url
	res.Data["Time"] = time.Now().Format("2006-01-02 15:04:05")
	return res
}

// 使用正则表达式从响应内容中提取 URL 和其他信息，生成新的请求，name字段用于指定新生成请求所使用的解析规则名称
func (c *Context) ParseJSReg(name string, reg string) ParseResult {
	re := regexp.MustCompile(reg)

	matches := re.FindAllSubmatch(c.Body, -1)
	result := ParseResult{}

	for _, m := range matches {
		u := string(m[1])
		result.Requests = append(
			result.Requests, &Request{
				Method:   "GET",
				Task:     c.Req.Task,
				Url:      u,
				Depth:    c.Req.Depth + 1,
				RuleName: name,
			})
	}
	return result
}

// 筛选出符合条件的请求url
func (c *Context) OutputJS(reg string) ParseResult {
	re := regexp.MustCompile(reg)
	ok := re.Match(c.Body)
	if !ok {
		return ParseResult{
			Items: []interface{}{},
		}
	}
	result := ParseResult{
		Items: []interface{}{c.Req.Url},
	}
	return result
}

// 表示一个具体的HTTP请求
type Request struct {
	unique   string // 表示请求的唯一识别码，用于去重
	Task     *Task  // 所属的任务
	Url      string // 请求的URL
	Method   string // 请求的方法，如GET、POST等
	Depth    int    // 请求的深度，用于控制爬取的最大深度
	Priority int    // 请求的优先级，用于控制请求的执行顺序
	RuleName string // 解析规则的名称
	TmpData  *Temp  // 临时数据
}

// 检查当前请求是否超过任务的最大请求深度
func (r *Request) Check() error {
	if r.Depth > r.Task.MaxDepth {
		return errors.New("Max depth limit reached")
	}
	return nil
}

// 用于生成请求的唯一识别码，用于去重
func (r *Request) Unique() string {
	block := md5.Sum([]byte(r.Url + r.Method))
	return hex.EncodeToString(block[:])
}

// 在工作协程发起请求之前，通过限速器限制请求速率，遍历多限速器的所有限速器，只有当所有限速器都满足的时候才能取得令牌，否则当前协程将会阻塞直到满足条件获取到令牌，并进行随机休眠
func (r *Request) Fetch() ([]byte, error) {
	// 在发起请求前，先检查是否达到了限速
	if err := r.Task.Limit.Wait(context.Background()); err != nil {
		return nil, err
	}
	// 随机休眠，模拟人类行为
	sleeptime := rand.Int63n(int64(r.Task.WaitTime * 1000)) // 生成一个随机整数
	time.Sleep(time.Duration(sleeptime) * time.Millisecond) // 让当前goroutine休眠随机的时间
	return r.Task.Fetcher.Get(r)
}
