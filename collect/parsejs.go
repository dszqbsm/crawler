package collect

// 专门处理需要JavaScript支持的解析场景

type (
	// 定义了一个爬虫任务模板
	TaskModle struct {
		Property             // 任务属性
		Root     string      `json:"root_script"` // 用于生成初始请求的JavaScript脚本
		Rules    []RuleModle `json:"rule"`        // 包含该任务下所有的解析规则
	}

	// 定义了一个解析规则模板
	RuleModle struct {
		Name      string `json:"name"`         // 规则名称
		ParseFunc string `json:"parse_script"` // 存储用于解析页面内容的JavaScript脚本
	}
)
