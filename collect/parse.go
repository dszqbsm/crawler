package collect

// 用于存放通用的解析规则和逻辑，适用于一般网页内容解析，不依赖特定的JavaScript执行环境

//采集规则树
type RuleTree struct {
	Root  func() ([]*Request, error) // 根节点(执行入口)，用于生成爬虫的种子网站（如豆瓣话题不同页面，豆瓣书籍不同页面）
	Trunk map[string]*Rule           // 规则哈希表存储当前任务所有规则
}

// 采集规则节点
type Rule struct {
	ItemFields []string                            // 用来表示当前输出数据的字段名
	ParseFunc  func(*Context) (ParseResult, error) // 内容解析函数
}
