package spider

import "encoding/json"

type ResourceSpec struct {
	ID           string // 资源id
	Name         string // 任务名
	AssignedNode string // 分配节点
	CreationTime int64  // 创建时间
}

/*
输入一个资源规格，输出一个json序列

该方法用于将资源规格编码为json序列，返回json序列的字符串
*/
func Encode(s *ResourceSpec) string {
	b, _ := json.Marshal(s)
	return string(b)
}

/*
输入一个json序列，输出一个资源规格和一个错误

该方法用于将json序列解码为资源规格，返回资源规格的指针和错误
*/
func Decode(ds []byte) (*ResourceSpec, error) {
	var s *ResourceSpec
	err := json.Unmarshal(ds, &s)
	return s, err
}
