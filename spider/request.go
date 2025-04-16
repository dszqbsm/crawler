package spider

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
)

// 表示一个具体的HTTP请求
type Request struct {
	Task     *Task  // 所属的任务
	Url      string // 请求的URL
	Method   string // 请求的方法，如GET、POST等
	Depth    int    // 请求的深度，用于控制爬取的最大深度
	Priority int    // 请求的优先级，用于控制请求的执行顺序
	RuleName string // 解析规则的名称
	TmpData  *Temp  // 临时数据
}

/*
无输入，输出一个错误

该方法用于验证请求的有效性，若当前请求深度超过任务配置中允许的最大深度，或任务已终止，则返回错误
*/
func (r *Request) Check() error {
	if r.Depth > r.Task.MaxDepth {
		return errors.New("Max depth limit reached")
	}

	if r.Task.Closed {
		return errors.New("task has Closed")
	}
	return nil
}

/*
无输入，输出一个字符串

该方法用于生成请求的唯一识别码，使用MD5哈希算法对请求的URL和方法进行哈希计算，然后将哈希值转换为十六进制字符串作为唯一识别码
*/
func (r *Request) Unique() string {
	block := md5.Sum([]byte(r.Url + r.Method))
	return hex.EncodeToString(block[:])
}
