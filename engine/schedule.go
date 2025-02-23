package engine

import (
	"runtime/debug"
	"sync"

	"github.com/dszqbsm/crawler/collect"
	"github.com/dszqbsm/crawler/collector"
	"github.com/dszqbsm/crawler/parse/doubanbook"
	"github.com/dszqbsm/crawler/parse/doubangroup"
	"github.com/dszqbsm/crawler/parse/doubangroupjs"
	"github.com/robertkrimen/otto"
	"go.uber.org/zap"
)

// 特殊函数，会在每个包被初始化时自动执行，用于初始化全局任务存储结构，添加普通爬虫任务，添加基于JavaScript的动态爬虫任务，添加豆瓣书任务
func init() {
	Store.Add(doubangroup.DoubangroupTask)
	Store.Add(doubanbook.DoubanBookTask)
	Store.AddJSTask(doubangroupjs.DoubangroupJSTask)
}

// 全局任务存储结构，用于动态添加爬虫任务
type CrawlerStore struct {
	list []*collect.Task          // 保存所有爬虫任务的列表
	Hash map[string]*collect.Task // 哈希表以任务名为键，用于快速查找任务
}

// 添加普通的爬虫任务，将新的爬虫任务添加进哈希表和爬虫任务列表
func (c *CrawlerStore) Add(task *collect.Task) {
	c.Hash[task.Name] = task
	c.list = append(c.list, task)
}

// 添加基于JavaScript的动态爬虫任务，即将任务存到列表和哈希表，只是多了动态生成爬虫任务的种子网站以及爬虫任务的规则
func (c *CrawlerStore) AddJSTask(m *collect.TaskModle) {
	task := &collect.Task{
		Property: m.Property,
	}

	// 动态生成任务的根请求结构
	task.Rule.Root = func() ([]*collect.Request, error) {
		vm := otto.New()              // JavaScript引擎初始化
		vm.Set("AddJsReq", AddJsReqs) // 将Go的函数AddJsReqs注入到JavaScript环境中，名称为AddJsReq，otto会自动将go函数封装成JavaScript函数，使得在JavaScript代码中能像调用普通JavaScript函数一样调用，当在JavaScript中调用这个封装后的函数时，otto会把JavaScript的参数转换成go函数能接受的参数类型然后调用go函数，再把go函数返回值转换未JavaScript能处理的值
		v, err := vm.Eval(m.Root)     // 执行JavaScript代码，即执行doubangroupjs.go中的Root JavaScript脚本，JavaScript脚本会生成初始请求数组并调用go函数AddJsReq
		if err != nil {
			return nil, err
		}
		e, err := v.Export() // 将JavaScript脚本执行结果v转换为go的接口类型
		if err != nil {
			return nil, err
		}
		return e.([]*collect.Request), nil // 类型断言，将e转换为[]*collect.Request类型
	}

	// 动态生成解析函数并添加到任务规则中
	for _, r := range m.Rules {
		// 动态生成解析函数
		// 匿名闭包函数，接受一个字符串参数parse，代表要执行的JavaScript代码，闭包函数返回一个新函数，这个新函数接受一个*collect.Context类型的参数ctx，返回一个collect.ParseResult类型和一个error类型的结果
		parseFunc := func(parse string) func(ctx *collect.Context) (collect.ParseResult, error) {
			return func(ctx *collect.Context) (collect.ParseResult, error) {
				vm := otto.New()
				vm.Set("ctx", ctx)
				v, err := vm.Eval(parse)
				if err != nil {
					return collect.ParseResult{}, err
				}
				e, err := v.Export()
				if err != nil {
					return collect.ParseResult{}, err
				}
				if e == nil {
					return collect.ParseResult{}, err
				}
				return e.(collect.ParseResult), err
			}
		}(r.ParseFunc)
		if task.Rule.Trunk == nil {
			task.Rule.Trunk = make(map[string]*collect.Rule, 0)
		}
		task.Rule.Trunk[r.Name] = &collect.Rule{
			ParseFunc: parseFunc,
		}
	}

	// 将任务添加到任务存储结构中
	c.Hash[task.Name] = task
	c.list = append(c.list, task)
}

// 动态任务生成辅助方法：转化JavaScript请求为Go的请求结构体
func AddJsReqs(jreqs []map[string]interface{}) []*collect.Request {
	reqs := make([]*collect.Request, 0)

	for _, jreq := range jreqs {
		req := &collect.Request{}
		u, ok := jreq["Url"].(string)
		if !ok {
			return nil
		}
		req.Url = u
		req.RuleName, _ = jreq["RuleName"].(string)
		req.Method, _ = jreq["Method"].(string)
		req.Priority, _ = jreq["Priority"].(int)
		reqs = append(reqs, req)
	}
	return reqs
}

// 动态任务生成辅助方法：转化单个JavaScript请求为Go的请求结构体
func AddJsReq(jreq map[string]interface{}) []*collect.Request {
	reqs := make([]*collect.Request, 0)
	req := &collect.Request{}
	u, ok := jreq["Url"].(string)
	if !ok {
		return nil
	}
	req.Url = u
	req.RuleName, _ = jreq["RuleName"].(string)
	req.Method, _ = jreq["Method"].(string)
	req.Priority, _ = jreq["Priority"].(int)
	reqs = append(reqs, req)
	return reqs
}

// 爬虫实例，管理整个爬取流程
type Crawler struct {
	out         chan collect.ParseResult // 用于传输解析结果的通道
	Visited     map[string]bool          // 记录已访问过的url
	VisitedLock sync.Mutex               // 访问记录锁

	failures    map[string]*collect.Request // 记录失败的请求，用于重试
	failureLock sync.Mutex                  // 失败记录锁

	options // 爬虫配置选项
}

// 启动爬虫，开启调度协程、创建多个工作携程，并调用结果处理方法处理解析结果
func (e *Crawler) Run() {
	go e.Schedule()
	for i := 0; i < e.WorkCount; i++ {
		go e.CreateWork()
	}
	e.HandleResult()
}

// 启动爬虫的调度流程，获取爬虫任务的种子网站的请求，并设置请求的任务为当前任务，并将请求发送到工作通道
func (e *Crawler) Schedule() {
	var reqs []*collect.Request
	for _, seed := range e.Seeds { // 遍历初始任务
		task := Store.Hash[seed.Name] // 获取任务实例
		task.Fetcher = seed.Fetcher   // 设置采集器
		task.Storage = seed.Storage
		task.Limit = seed.Limit
		task.Logger = e.Logger
		rootreqs, err := task.Rule.Root() // 生成根请求
		if err != nil {
			e.Logger.Error("get root failed",
				zap.Error(err),
			)
			continue
		}
		for _, req := range rootreqs {
			req.Task = task
		}
		reqs = append(reqs, rootreqs...)
	}
	go e.scheduler.Schedule()
	go e.scheduler.Push(reqs...)
}

// 工作协程的核心逻辑，用于从通道中获取请求，发送请求并获取响应，并在获取规则后对响应进行解析，若结果集中包含子请求则将子请求发送到工作通道，最后将结果发送到结果处理通道
func (s *Crawler) CreateWork() {
	// 使用defer和recover处理panic异常，防止程序崩溃
	defer func() {
		if err := recover(); err != nil {
			s.Logger.Error("worker panic",
				zap.Any("err", err),
				zap.String("stack", string(debug.Stack())))
		}
	}()
	for {
		req := s.scheduler.Pull()           // 从通道中获取请求
		if err := req.Check(); err != nil { // 检查当前请求是否可用
			s.Logger.Error("check failed",
				zap.Error(err),
			)
			continue
		}
		if !req.Task.Reload && s.HasVisited(req) { // 检查请求是否允许重新加载，不允许且请求已经被访问过则跳过当前循环，若未被访问过则将请求标记为已访问
			s.Logger.Debug("request has visited",
				zap.String("url:", req.Url),
			)
			continue
		}
		s.StoreVisited(req)

		body, err := req.Fetch() // 发送请求并获取响应
		if err != nil {
			s.Logger.Error("can't fetch ",
				zap.Error(err),
				zap.String("url", req.Url),
			)
			s.SetFailure(req)
			continue
		}

		if len(body) < 6000 {
			s.Logger.Error("can't fetch ",
				zap.Int("length", len(body)),
				zap.String("url", req.Url),
			)
			s.SetFailure(req)
			continue
		}

		rule := req.Task.Rule.Trunk[req.RuleName] // 根据请求的规则名称从规则树中获取规则

		result, err := rule.ParseFunc(&collect.Context{ // 调用规则的解析函数处理响应
			body,
			req,
		})

		if err != nil {
			s.Logger.Error("ParseFunc failed ",
				zap.Error(err),
				zap.String("url", req.Url),
			)
			continue
		}

		if len(result.Requests) > 0 { // 如果解析结果中包含子请求，则将子请求推送到调度器中
			go s.scheduler.Push(result.Requests...)
		}

		s.out <- result // 将解析结果推送到out通道
	}
}

// 处理解析结果的方法，从out通道接收解析结果，遍历输出结果
func (s *Crawler) HandleResult() {
	for {
		select {
		case result := <-s.out:
			for _, item := range result.Items { // 遍历结果集中的条目
				switch d := item.(type) { // 做类型断言，若为*collector.DataCell类型，则调用任务的存储引擎保存数据
				case *collector.DataCell:
					name := d.GetTaskName()
					task := Store.Hash[name]
					task.Storage.Save(d)
				}
				s.Logger.Sugar().Info("get result: ", item)
			}
		}
	}
}

// 用于判断某个请求是否已经被访问过，通过加锁保证线程安全
func (e *Crawler) HasVisited(r *collect.Request) bool {
	e.VisitedLock.Lock()
	defer e.VisitedLock.Unlock()
	unique := r.Unique()
	return e.Visited[unique]
}

// 用于将请求标记为已访问，通过加锁保证线程安全
func (e *Crawler) StoreVisited(reqs ...*collect.Request) {
	e.VisitedLock.Lock()
	defer e.VisitedLock.Unlock()

	for _, r := range reqs {
		unique := r.Unique()
		e.Visited[unique] = true
	}
}

// 用于处理失败的请求，通过加锁保证线程安全，若该请求不允许重新加载则将请求从已访问记录中移除，如果请求允许重试，会将其重新发送到工作协程
func (e *Crawler) SetFailure(req *collect.Request) {
	if !req.Task.Reload { // 若不允许重新加载
		e.VisitedLock.Lock()
		unique := req.Unique()
		delete(e.Visited, unique)
		e.VisitedLock.Unlock()
	}
	e.failureLock.Lock()
	defer e.failureLock.Unlock()
	if _, ok := e.failures[req.Unique()]; !ok {
		// 首次失败时，再重新执行一次
		e.failures[req.Unique()] = req
		e.scheduler.Push(req)
	}
	// todo: 失败2次，加载到失败队列中
}

// 调度器：起任务调度中心的作用，负责接收新的请求、对请求进行优先级分类、将请求分发给工作线程进行处理
type Schedule struct {
	requestCh   chan *collect.Request // 任务提交通道，用于接收新的请求
	workerCh    chan *collect.Request // 工作通道，工作线程从该通道获取任务
	priReqQueue []*collect.Request    // 优先队列
	reqQueue    []*collect.Request    // 普通队列
	Logger      *zap.Logger           // 日志器
}

// 新请求通道和工作通道发送和接收的都是请求，而不是任务，任务对于一个爬虫任务来说是固定的，是爬虫的固有属性，从而将处理请求和任务解耦

// 将请求发送到新请求通道
func (s *Schedule) Push(reqs ...*collect.Request) {
	for _, req := range reqs {
		s.requestCh <- req
	}
}

// 从工作通道中接收一个请求
func (s *Schedule) Pull() *collect.Request {
	r := <-s.workerCh
	return r
}

func (s *Schedule) Output() *collect.Request {
	r := <-s.workerCh
	return r
}

// 负责请求调度，维护两个队列，优先处理高优先级队列中的请求，当收到新请求时，根据请求的优先级将其添加到对应的队列中
func (s *Schedule) Schedule() {
	var req *collect.Request
	var ch chan *collect.Request
	for {
		if req == nil && len(s.priReqQueue) > 0 {
			req = s.priReqQueue[0]
			s.priReqQueue = s.priReqQueue[1:]
			ch = s.workerCh
		}
		if req == nil && len(s.reqQueue) > 0 {
			req = s.reqQueue[0]
			s.reqQueue = s.reqQueue[1:]
			ch = s.workerCh
		}

		select {
		case r := <-s.requestCh:
			if r.Priority > 0 {
				s.priReqQueue = append(s.priReqQueue, r)
			} else {
				s.reqQueue = append(s.reqQueue, r)
			}
		case ch <- req:
			req = nil
			ch = nil
		}
	}
}

// 全局爬虫存储实例
var Store = &CrawlerStore{
	list: []*collect.Task{},
	Hash: map[string]*collect.Task{},
}

// 为调度器提供了统一的接口规范，使得不同调度器实现都要遵循这些方法
type Scheduler interface {
	Schedule()                // 启动调度器
	Push(...*collect.Request) // 向调度器提交新的请求
	Pull() *collect.Request   // 从调度器中获取一个请求
}

// 创建并初始化一个Crawler爬虫实例，通过传入不同的配置选项，可以灵活配置爬虫的行为
func NewEngine(opts ...Option) *Crawler {
	options := defaultOptions
	for _, opt := range opts {
		opt(&options)
	}
	e := &Crawler{}
	e.Visited = make(map[string]bool, 100)
	e.out = make(chan collect.ParseResult)
	e.failures = make(map[string]*collect.Request)
	e.options = options
	return e
}

// 创建并初始化一个schedule调度器实例
func NewSchedule() *Schedule {
	s := &Schedule{}
	requestCh := make(chan *collect.Request)
	workerCh := make(chan *collect.Request)
	s.requestCh = requestCh
	s.workerCh = workerCh
	return s
}

// 根据任务名和规则名获取对应的字段列表
func GetFields(taskName string, ruleName string) []string {
	return Store.Hash[taskName].Rule.Trunk[ruleName].ItemFields
}
