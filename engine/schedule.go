package engine

import (
	"fmt"

	"github.com/dszqbsm/crawler/collect"
	"go.uber.org/zap"
)

type Schedule struct {
	requestCh chan *collect.Request
	workerCh  chan *collect.Request
	out       chan collect.ParseResult
	options
}

func NewSchedule(opts ...Option) *Schedule {
	options := defaultOptions // 默认配置只设置了日志组件，通过opts闭包函数动态添加其他配置
	for _, opt := range opts {
		opt(&options)
	}
	s := &Schedule{}
	s.options = options
	return s
}

func (s *Schedule) Run() {
	// 无缓冲通道，向无缓冲通道发送数据时，发送操作会阻塞，直到有接收方准备好接收数据，从无缓冲通道接收数据时同样会阻塞直到有发送方发送数据
	requestCh := make(chan *collect.Request)
	workerCh := make(chan *collect.Request)
	out := make(chan collect.ParseResult)
	s.requestCh = requestCh
	s.workerCh = workerCh
	s.out = out
	go s.Schedule() // 启动调度协程
	// fmt.Println("调度协程启动成功")
	for i := 0; i < s.WorkCount; i++ {
		//fmt.Println("启动第", i, "个工作协程")
		go s.CreateWork() // 启动多个工作协程
	}
	//fmt.Println("工作协程启动完成")
	s.HandleResult() // 结果处理逻辑
}

func (s *Schedule) Schedule() {
	/* if len(s.Seeds) == 0 {
		s.Logger.Fatal("调度器启动失败，初始任务队列为空")
	} */
	var reqQueue = s.Seeds
	go func() {
		for {
			var req *collect.Request
			var ch chan *collect.Request

			if len(reqQueue) > 0 {
				req = reqQueue[0]
				reqQueue = reqQueue[1:]
				ch = s.workerCh // ch 和 s.workerCh 指向同一个通道
			}
			// 因为定义的通道是无缓冲通道，因此会阻塞直到发送方发送或接收方准备好
			// 向通道发送数据实际上是将数据放入通道的队列中，等待接收方从通道中取出数据
			select {
			case r := <-s.requestCh: // 接收新任务，将其加入任务队列，当s.requestCh有消息时会执行
				reqQueue = append(reqQueue, r)

			case ch <- req: // 将任务发送给工作协程
				// fmt.Println("发送任务", req.Url)
			}
		}
	}()
}

// 除了采集所有话题url，还会过滤每个话题url，找到带有阳台字样的话题url
// 同样是有广度优先的思想，会对获取到的话题url继续放入队列，此时捕获不到话题url
func (s *Schedule) CreateWork() {
	for {
		r := <-s.workerCh
		// fmt.Println("工作协程接收到任务", r.Url)
		body, err := s.Fetcher.Get(r)
		if len(body) < 6000 {
			s.Logger.Error("can't fetch ",
				zap.Int("length", len(body)),
				zap.String("url", r.Url),
			)
		}
		if err != nil {
			s.Logger.Error("can't fetch ",
				zap.Error(err),
				zap.String("url", r.Url),
			)
			continue
		}
		//fmt.Println("发送请求得到的响应为", string(body))
		result := r.ParseFunc(body, r)
		//res := result.ParseFunc(body, r)
		s.out <- result
		// fmt.Println("工作协程完成任务", r.Url)
	}
}

func (s *Schedule) HandleResult() {
	for {
		select {
		case result := <-s.out:
			fmt.Println("结果处理协程接收到结果", result)
			for _, req := range result.Requests {
				fmt.Println("将话题url添加到任务队列", req.Url)
				s.requestCh <- req
			}
			for _, item := range result.Items {
				// todo: store
				s.Logger.Sugar().Info("get result: ", item)
			}
		}
	}
}
