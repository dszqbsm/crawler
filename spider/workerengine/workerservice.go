package workerengine

import (
	"runtime/debug"
	"strings"
	"sync"

	"github.com/dszqbsm/crawler/spider"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// 工作节点提供的服务，

type WorkerService interface {
	Run(cluster bool)
	LoadResource() error
	WatchResource()
}

type workerService struct {
	out     chan spider.ParseResult
	rlock   sync.Mutex
	etcdCli *clientv3.Client
	options
}

/*
输入多个配置选项，输出一个workerService实例和错误

该方法用于创建一个新的工作节点实例，初始化工作节点的配置选项、采集器和存储器，并返回工作节点实例的指针和错误
*/
func NewWorkerService(opts ...Option) (*workerService, error) {
	options := defaultOptions
	for _, opt := range opts {
		opt(&options)
	}

	e := &workerService{}
	e.out = make(chan spider.ParseResult)
	e.options = options

	// 任务加上默认的采集器与存储器
	for _, task := range spider.TaskStore.List {
		task.Fetcher = e.Fetcher
		task.Storage = e.Storage
	}

	return e, nil
}

/*
输入一个布尔值，表示是否为集群模式，无输出

该方法用于启动工作节点，非集群模式要加载本地种子资源，从etcd加载分配给该节点的资源，开启协程监听资源变化，开启协程启动调度器，创建多个工作协程处理请求，并调用结果处理方法处理解析结果
*/
func (c *workerService) Run(cluster bool) {
	if !cluster {
		c.handleSeeds() // 单机模式加载本地种子任务
	}
	// 集群模式则跳过本地种子，完全依赖LoadResource()从etcd加载分配的任务
	c.LoadResource()          // 加载资源
	go c.WatchResource()      // 监听资源变化
	go c.scheduler.Schedule() // 启动调度器，监听新请求分发请求
	for i := 0; i < c.WorkCount; i++ {
		go c.CreateWork() // 创建多个工作协程处理请求
	}
	c.HandleResult() // 处理解析结果
}

/*
无输入，输出一个错误

该方法用于从etcd加载资源列表，筛选属于当前节点的资源，并存储到本地资源仓库，然后启动每个资源的任务
*/
func (c *workerService) LoadResource() error {
	resources := make(map[string]*spider.ResourceSpec)
	resourceSpecs, err := c.resourceRegistry.GetResources() // 从etcd加载资源列表
	if err != nil {
		return err
	}

	// 筛选属于当前节点的资源
	for _, r := range resourceSpecs {
		id := getID(r.AssignedNode)
		if len(id) > 0 && c.id == id {
			resources[r.Name] = r
		}
	}

	// 存储到本地资源仓库
	c.Logger.Info("leader init load resource", zap.Int("lenth", len(resources)))
	c.resourceRepository.Set(resources)
	for _, r := range resources {
		c.runTasks(r.Name) // 启动每个资源的任务
	}

	return nil
}

/*
无输入，无输出

该方法用于监听etcd资源变更事件，根据事件类型执行相应的操作，如新增或更新资源时启动任务，删除资源时停止并删除任务
*/
func (c *workerService) WatchResource() {
	// 监听etcd资源变更事件
	watch := c.resourceRegistry.WatchResources()
	for w := range watch {
		if w.Canceled {
			c.Logger.Error("watch resource canceled")
			return
		}
		switch w.Typ {
		case spider.EventTypePut: // 新增或更新资源
			c.rlock.Lock()
			if w.Res != nil {
				c.runTasks(w.Res.Name) // 启动或更新任务
			}
			c.rlock.Unlock()
		case spider.EventTypeDelete: // 删除资源
			c.rlock.Lock()
			if w.Res != nil {
				c.deleteTasks(w.Res.Name) // 停止并删除任务
			}
			c.rlock.Unlock()
		}
	}
}

/*
输入一个任务名，无输出

该方法用于停止并删除指定任务，并从本地资源仓库中删除该任务，先从任务仓库中查找任务，然后将任务的关闭标志设置为true，最后从本地资源仓库中删除该任务
*/
func (c *workerService) deleteTasks(taskName string) {
	t, ok := spider.TaskStore.Hash[taskName]
	if !ok {
		c.Logger.Error("can not find preset tasks", zap.String("task name", taskName))
		return
	}
	t.Closed = true

	c.resourceRepository.Delete(taskName)
}

/*
输入一个任务名，无输出

该方法用于启动指定任务，首先从资源仓库中查找指定资源，如果未找到则返回；如果已找到，则从任务仓库中查找指定任务，如果未找到则返回；找到任务后，将任务的关闭标志设置为false，并生成初始请求，将请求推送到调度器中
*/
func (c *workerService) runTasks(name string) {
	// 避免重复运行任务
	if c.resourceRepository.HasResource(name) {
		c.Logger.Info("task has runing", zap.String("name", name))
		return
	}

	t, ok := spider.TaskStore.Hash[name]
	if !ok {
		c.Logger.Error("can not find preset tasks", zap.String("task name", name))
		return
	}
	t.Closed = false
	// 生成初始请求
	res, err := t.Rule.Root()

	if err != nil {
		c.Logger.Error("get root failed",
			zap.Error(err),
		)
		return
	}

	for _, req := range res {
		req.Task = t
	}

	c.scheduler.Push(res...)
}

// 用于加载本地种子任务
/*
无输入，无输出

该方法用于加载本地种子任务，遍历预设的种子任务，从任务仓库中查找预配置任务，并将预配置任务的规则绑定到当前任务实例，然后生成当前任务的根请求，即初始请求，并未每个根请求绑定所属的任务，将初始请求推入调度器中
*/
func (c *workerService) handleSeeds() {
	var reqs []*spider.Request
	// 遍历预设的种子任务
	for _, task := range c.Seeds {
		// 从爬虫任务仓库查找预配置任务
		t, ok := spider.TaskStore.Hash[task.Name]
		if !ok {
			c.Logger.Error("can not find preset tasks", zap.String("task name", task.Name))
			continue
		}
		// 将预配置任务的规则绑定到当前任务实例
		task.Rule = t.Rule
		// 生成当前任务的根请求，即初始请求
		rootreqs, err := task.Rule.Root()

		if err != nil {
			c.Logger.Error("get root failed",
				zap.Error(err),
			)
			continue
		}

		// 为每个根请求绑定所属任务
		for _, req := range rootreqs {
			req.Task = task
		}

		reqs = append(reqs, rootreqs...) // 将初始请求推入调度队列
	}
	go c.scheduler.Push(reqs...)
}

/*
无输入，无输出

该方法用于创建一个工作节点，从调度器中获取请求，对请求进行校验，若请求已访问过则跳过，若请求未访问过则将请求添加到已访问列表中，然后调用请求的抓取器获取请求的响应体，若响应体长度小于6000字节则跳过，并将请求设置为失败请求，否则调用请求的规则解析器解析响应体，将解析结果推入结果通道中
*/
func (c *workerService) CreateWork() {
	defer func() {
		if err := recover(); err != nil {
			c.Logger.Error("worker panic",
				zap.Any("err", err),
				zap.String("stack", string(debug.Stack())))
		}
	}()

	for {
		req := c.scheduler.Pull()
		if err := req.Check(); err != nil {
			c.Logger.Debug("check failed",
				zap.Error(err),
			)

			continue
		}

		if !req.Task.Reload && c.reqRepository.HasVisited(req) {
			c.Logger.Debug("request has visited",
				zap.String("url:", req.Url),
			)

			continue
		}

		c.reqRepository.AddVisited(req)

		body, err := req.Task.Fetcher.Get(req)
		if err != nil {
			c.Logger.Error("can't fetch ",
				zap.Error(err),
				zap.String("url", req.Url),
			)
			c.SetFailure(req)

			continue
		}

		if len(body) < 6000 {
			c.Logger.Error("can't fetch ",
				zap.Int("length", len(body)),
				zap.String("url", req.Url),
			)
			c.SetFailure(req)

			continue
		}

		rule := req.Task.Rule.Trunk[req.RuleName]
		ctx := &spider.Context{
			Body: body,
			Req:  req,
		}
		result, err := rule.ParseFunc(ctx)

		if err != nil {
			c.Logger.Error("ParseFunc failed ",
				zap.Error(err),
				zap.String("url", req.Url),
			)

			continue
		}

		if len(result.Requests) > 0 {
			go c.scheduler.Push(result.Requests...)
		}

		c.out <- result
	}
}

/*
无输入，无输出

该方法用于处理解析结果，循环阻塞从结果通道中获取解析结果，遍历解析结果中的每个数据项，根据数据项的类型进行相应的处理，若数据项为DataCell类型，则调用数据项的任务的存储器将数据项保存到存储器中，若数据项为其他类型，则打印数据项的信息
*/
func (c *workerService) HandleResult() {
	for result := range c.out {
		for _, item := range result.Items {
			switch d := item.(type) {
			case *spider.DataCell:
				if err := d.Task.Storage.Save(d); err != nil {
					c.Logger.Error("")
				}
			}
			c.Logger.Sugar().Info("get result: ", item)
		}
	}
}

/*
输入一个请求，无输出

该方法用于将请求设置为失败请求，若请求不允许重新加载，则将请求从已访问列表中删除，若请求未达最大重试次数，则重新推送到调度器中
*/
func (c *workerService) SetFailure(req *spider.Request) {
	if !req.Task.Reload {
		c.reqRepository.DeleteVisited(req)
	}

	if c.reqRepository.AddFailures(req) {
		c.scheduler.Push(req)
	}
}

/*
输入一个分配节点，输出一个节点id

该方法用于从分配节点中提取节点id，将分配节点按照"|"分割成字符串切片，若切片长度小于2，则返回空字符串，否则返回切片的第一个元素作为节点id
*/
func getID(assignedNode string) string {
	s := strings.Split(assignedNode, "|")
	if len(s) < 2 {
		return ""
	}
	return s[0]
}
