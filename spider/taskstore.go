package spider

import (
	"github.com/robertkrimen/otto"
)

// TaskStore is a global instace
var (
	TaskStore = &taskStore{
		List: []*Task{},
		Hash: map[string]*Task{},
	}
)

type taskStore struct {
	List []*Task
	Hash map[string]*Task
}

/*
输入一个任务，无输出

该方法用于将一个任务添加到任务存储中，将任务添加到任务列表和任务哈希表中
*/
func (c *taskStore) Add(task *Task) {
	c.Hash[task.Name] = task
	c.List = append(c.List, task)
}

/*
输入一个任务模型，无输出

该方法用于将一个任务模型转换为任务，通过js虚拟机动态定义Root规则和子规则，并将任务添加到任务存储中，将任务添加到任务列表和任务哈希表中
*/
func (c *taskStore) AddJSTask(m *TaskModle) {
	task := &Task{
		//Property: m.Property,
	}
	// 定义 Root 规则（生成初始请求的 JS 逻辑）：通过 JS 生成初始请求列表
	task.Rule.Root = func() ([]*Request, error) {
		vm := otto.New()                                      // 创建 JS 虚拟机
		if err := vm.Set("AddJsReq", AddJsReqs); err != nil { // 注入 Go 函数到 JS
			return nil, err
		}

		v, err := vm.Eval(m.Root) // 执行 JS 代码

		if err != nil {
			return nil, err
		}

		e, err := v.Export() // 导出结果

		if err != nil {
			return nil, err
		}

		return e.([]*Request), nil
	}
	// 定义子规则（Trunk）：通过 JS 定义每个子页面的解析逻辑
	for _, r := range m.Rules {
		paesrFunc := func(parse string) func(ctx *Context) (ParseResult, error) {
			return func(ctx *Context) (ParseResult, error) {
				vm := otto.New()
				if err := vm.Set("ctx", ctx); err != nil { // 注入上下文到 JS
					return ParseResult{}, err
				}

				v, err := vm.Eval(parse) // 执行 JS 解析逻辑

				if err != nil {
					return ParseResult{}, err
				}

				e, err := v.Export()

				if err != nil {
					return ParseResult{}, err
				}

				if e == nil {
					return ParseResult{}, err
				}

				return e.(ParseResult), err
			}
		}(r.ParseFunc)

		if task.Rule.Trunk == nil {
			task.Rule.Trunk = make(map[string]*Rule, 0)
		}

		task.Rule.Trunk[r.Name] = &Rule{
			ParseFunc: paesrFunc,
		}
	}

	c.Hash[task.Name] = task
	c.List = append(c.List, task)
}

/*
输入一个请求列表，输出一个请求列表

该方法用于将 JS 环境中的请求描述转换为 Go 的 Request 对象
*/
func AddJsReqs(jreqs []map[string]interface{}) []*Request {
	reqs := make([]*Request, 0)

	for _, jreq := range jreqs {
		req := &Request{}
		u, ok := jreq["URL"].(string)

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

/*
输入一个请求列表，输出一个请求列表

该方法用于将 JS 环境中的请求描述转换为 Go 的 Request 对象
*/
func AddJsReq(jreq map[string]interface{}) []*Request {
	reqs := make([]*Request, 0)
	req := &Request{}
	u, ok := jreq["URL"].(string)

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

/*
输入任务名称和规则名称，输出字段列表

该方法用于获取指定任务和规则的字段列表
*/
func GetFields(taskName string, ruleName string) []string {
	return TaskStore.Hash[taskName].Rule.Trunk[ruleName].ItemFields
}
