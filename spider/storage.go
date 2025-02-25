package spider

// 数据单元
type DataCell struct {
	Task *Task
	Data map[string]interface{} // 键为字符串类型，值为任意类型的映射
}

// 获取数据单元的表名
func (d *DataCell) GetTableName() string {
	return d.Data["Table"].(string)
}

// 获取数据单元的任务名
func (d *DataCell) GetTaskName() string {
	return d.Data["Task"].(string)
}

// 定义了存储引擎的统一规范
type Storage interface {
	Save(datas ...*DataCell) error
}
