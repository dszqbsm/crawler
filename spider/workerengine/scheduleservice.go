package workerengine

import (
	"github.com/dszqbsm/crawler/spider"
	"go.uber.org/zap"
)

// 调度器提供的功能，ok理解

// 为调度器提供了统一的接口规范，使得不同调度器实现都要遵循这些方法
type Scheduler interface {
	/*
		无输入，无输出

		该方法用于启动调度器，维护两个队列，优先处理优先队列中的请求，并进行请求校验，当收到新请求时，根据请求的优先级将其添加到对应的队列中，并将请求分发给工作线程进行处理
	*/
	Schedule() // 启动调度器
	/*
	   输入一个或多个请求，无输出

	   该方法用于将一个或多个请求发送到新请求通道，用于接收新的请求
	*/
	Push(...*spider.Request) // 向调度器提交新的请求
	/*
	   无输入，输出一个请求

	   该方法用于从工作通道中接收一个请求，用于将请求分发给工作线程进行处理
	*/
	Pull() *spider.Request // 从调度器中获取一个请求
}

// 调度器：起任务调度中心的作用，负责接收新的请求、对请求进行优先级分类、将请求分发给工作线程进行处理
type Schedule struct {
	requestCh   chan *spider.Request // 任务提交通道，用于接收新的请求
	workerCh    chan *spider.Request // 工作通道，工作线程从该通道获取任务
	priReqQueue []*spider.Request    // 优先队列
	reqQueue    []*spider.Request    // 普通队列
	Logger      *zap.Logger          // 日志器
}

/*
无输入，输出一个schedule实例

该方法用于创建一个新的调度器实例，初始化新请求通道和工作通道，并返回调度器实例的指针。
*/
func NewSchedule() *Schedule {
	s := &Schedule{}
	requestCh := make(chan *spider.Request)
	workerCh := make(chan *spider.Request)
	s.requestCh = requestCh
	s.workerCh = workerCh

	return s
}

/*
输入一个或多个请求，无输出

该方法用于将一个或多个请求发送到新请求通道，用于接收新的请求
*/
func (s *Schedule) Push(reqs ...*spider.Request) {
	for _, req := range reqs {
		s.requestCh <- req
	}
}

/*
无输入，输出一个请求

该方法用于从工作通道中接收一个请求，用于将请求分发给工作线程进行处理
*/
func (s *Schedule) Pull() *spider.Request {
	r := <-s.workerCh

	return r
}

/*
无输入，无输出

该方法用于启动调度器，维护两个队列，优先处理优先队列中的请求，并进行请求校验，当收到新请求时，根据请求的优先级将其添加到对应的队列中，并将请求分发给工作线程进行处理
*/
func (s *Schedule) Schedule() {
	var ch chan *spider.Request

	var req *spider.Request

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

		// 请求校验
		if req != nil {
			if err := req.Check(); err != nil {
				zap.L().Debug("check failed",
					zap.Error(err),
				)
				// 重置请求和通道，并跳过当前请求
				req = nil
				ch = nil
				continue
			}
		}

		select {
		case r := <-s.requestCh:
			if r.Priority > 0 {
				s.priReqQueue = append(s.priReqQueue, r)
			} else {
				s.reqQueue = append(s.reqQueue, r)
			}
		case ch <- req:
			// 会尝试将req发送到ch通道，若发送成功即有工作线程在等待接收请求，则重置req和ch表示当前请求已经分配完毕；
			// 若ch为nil则说明没有需要往工作通道发送的请求，会忽略ch <- req，阻塞等待r := <-s.requestCh，等待接收新请求进行分配
			req = nil
			ch = nil
		}
	}
}
