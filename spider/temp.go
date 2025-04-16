package spider

// Temp结构体为爬虫项目提供了一个灵活的临时数据存储和管理机制

type Temper interface {
	/*
	   输入一个key键值，输出一个interface类型的值

	   该方法用于根据给定的键值获取临时缓存数据，如果存在则返回对应的值，否则返回nil
	*/
	Get(key string) interface{}
	/*
	   输入一个key键值和一个interface类型的值，输出一个error类型的值

	   该方法用于将给定的键值和对应的值存储到临时缓存数据中，如果存储成功则返回nil，否则返回一个error类型的值
	*/
	Set(key string, value interface{}) error
}

// 用于管理临时缓存数据
type Temp struct {
	data map[string]interface{}
}

/*
输入一个key键值，输出一个interface类型的值

该方法用于根据给定的键值获取临时缓存数据，如果存在则返回对应的值，否则返回nil
*/
func (t *Temp) Get(key string) interface{} {
	return t.data[key]
}

/*
输入一个key键值和一个interface类型的值，输出一个error类型的值

该方法用于将给定的键值和对应的值存储到临时缓存数据中，如果存储成功则返回nil，否则返回一个error类型的值
*/
func (t *Temp) Set(key string, value interface{}) error {
	if t.data == nil {
		t.data = make(map[string]interface{}, 8)
	}
	t.data[key] = value
	return nil
}
