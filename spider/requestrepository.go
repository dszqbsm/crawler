package spider

import "sync"

type ReqHistoryRepository interface {
	/*
	   输入一个或多个请求，无输出

	   该方法用于将一个或多个请求添加到已访问的请求列表中
	*/
	AddVisited(reqs ...*Request)
	/*
	   输入一个请求，无输出

	   该方法用于将一个请求从已访问的请求列表中删除
	*/
	DeleteVisited(req *Request)
	/*
	   输入一个请求，输出一个布尔值，true表示首次失败允许重试，false表示已失败过

	   该方法用于将一个请求添加到失败的请求列表中，若该请求已存在于失败的请求列表中，则返回false，否则返回true
	*/
	AddFailures(req *Request) bool
	/*
	   输入一个请求，无输出

	   该方法用于将一个请求从失败的请求列表中删除
	*/
	DeleteFailures(req *Request)
	/*
	   输入一个请求，输出一个布尔值

	   该方法用于判断请求是否已经被访问过，若已访问过则返回true，否则返回false
	*/
	HasVisited(req *Request) bool
}

type reqHistory struct {
	Visited     map[string]bool
	VisitedLock sync.Mutex

	failures    map[string]*Request // 失败请求id -> 失败请求
	failureLock sync.Mutex
}

/*
无输入，输出一个ReqHistoryRepository实例

该方法用于创建一个新的ReqHistoryRepository实例，并初始化
*/
func NewReqHistoryRepository() ReqHistoryRepository {
	r := &reqHistory{}
	r.Visited = make(map[string]bool, 100)
	r.failures = make(map[string]*Request, 100)
	return r
}

/*
输入一个请求，输出一个布尔值

该方法用于判断请求是否已经被访问过，若已访问过则返回true，否则返回false
*/
func (r *reqHistory) HasVisited(req *Request) bool {
	r.VisitedLock.Lock()
	defer r.VisitedLock.Unlock()

	unique := req.Unique()

	return r.Visited[unique]
}

/*
输入一个或多个请求，无输出

该方法用于将一个或多个请求添加到已访问的请求列表中
*/
func (r *reqHistory) AddVisited(reqs ...*Request) {
	r.VisitedLock.Lock()
	defer r.VisitedLock.Unlock()

	for _, req := range reqs {
		unique := req.Unique()
		r.Visited[unique] = true
	}
}

/*
输入一个请求，无输出

该方法用于将一个请求从已访问的请求列表中删除
*/
func (r *reqHistory) DeleteVisited(req *Request) {
	r.VisitedLock.Lock()
	defer r.VisitedLock.Unlock()
	unique := req.Unique()
	delete(r.Visited, unique)
}

/*
输入一个请求，输出一个布尔值，true表示首次失败允许重试，false表示已失败过

该方法用于将一个请求添加到失败的请求列表中，若该请求已存在于失败的请求列表中，则返回false，否则返回true
*/
func (r *reqHistory) AddFailures(req *Request) bool {

	first := true
	if !req.Task.Reload {
		r.DeleteVisited(req)
	}

	r.failureLock.Lock()
	defer r.failureLock.Unlock()

	if _, ok := r.failures[req.Unique()]; !ok {
		r.failures[req.Unique()] = req
	} else {
		first = false
	}

	return first

}

/*
输入一个请求，无输出

该方法用于将一个请求从失败的请求列表中删除
*/
func (r *reqHistory) DeleteFailures(req *Request) {
	r.failureLock.Lock()
	defer r.failureLock.Unlock()

	delete(r.failures, req.Unique())
}
