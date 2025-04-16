package spider

type DataRepository interface {
	Save(datas ...*DataCell) error
}

type DataCell struct {
	Task *Task
	Data map[string]interface{}
}

/*
无输入，输出一个字符串

该方法用于获取数据单元的表名，返回数据单元的表名
*/
func (d *DataCell) GetTableName() string {
	return d.Data["Task"].(string)
}

/*
无输入，输出一个字符串

该方法用于获取数据单元的任务名称，返回数据单元的任务名称
*/
func (d *DataCell) GetTaskName() string {
	return d.Data["Task"].(string)
}
