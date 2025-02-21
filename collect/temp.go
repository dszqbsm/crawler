package collect

// Temp结构体为爬虫项目提供了一个灵活的临时数据存储和管理机制

// 用于管理临时缓存数据
type Temp struct {
	data map[string]interface{}
}

// 根据key键值获取临时缓存数据
func (t *Temp) Get(key string) interface{} {
	return t.data[key]
}

// 将给定的键值存储临时缓存数据
func (t *Temp) Set(key string, value interface{}) error {
	if t.data == nil {
		t.data = make(map[string]interface{}, 8)
	}
	t.data[key] = value
	return nil
}
