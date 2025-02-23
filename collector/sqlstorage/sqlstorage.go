package sqlstorage

// 定义了SqlStore结构体及其方法，主要用于将爬虫收集到的数据存储到SQL数据库中，提供了分批缓存、表创建、数据插入等功能，支持灵活的配置选项

import (
	"encoding/json"

	"github.com/dszqbsm/crawler/collector"
	"github.com/dszqbsm/crawler/engine"
	"github.com/dszqbsm/crawler/sqldb"
	"go.uber.org/zap"
)

type SqlStore struct {
	dataDocker  []*collector.DataCell // 用于缓存待插入数据库的数据单元
	columnNames []sqldb.Field         // 存储表的列名信息
	db          sqldb.DBer            // 数据库操作接口
	Table       map[string]struct{}   // 用于存储已创建的表名
	options                           // 存储SqlStore的配置选项
}

// SqlStore的构造函数，接受一系列配置选项，返回一个SqlStore实例
func New(opts ...Option) (*SqlStore, error) {
	options := defaultOptions
	for _, opt := range opts {
		opt(&options)
	}
	s := &SqlStore{}
	s.options = options
	s.Table = make(map[string]struct{})
	var err error
	s.db, err = sqldb.New(
		sqldb.WithConnUrl(s.sqlUrl),
		sqldb.WithLogger(s.logger),
	)
	if err != nil {
		return nil, err
	}

	return s, nil
}

// 用于保存数据单元到SqlStore中
func (s *SqlStore) Save(dataCells ...*collector.DataCell) error {
	for _, cell := range dataCells { // 遍历传入的数据单元，检查对应的表是否已经创建，若表不存在则获取列名并创建表
		name := cell.GetTableName()
		if _, ok := s.Table[name]; !ok {
			// 创建表
			columnNames := getFields(cell)

			err := s.db.CreateTable(sqldb.TableData{
				TableName:   name,
				ColumnNames: columnNames,
				AutoKey:     true,
			})
			if err != nil {
				s.logger.Error("create table falied", zap.Error(err))
			}
			s.Table[name] = struct{}{} // 在s.Table映射中添加一个键为name，值为空结构体示例的键值对，用于标记该列已经创建
		}
		if len(s.dataDocker) >= s.BatchCount { // 若数据单元数量达到批量处理数量，则调用Flush方法将数据插入数据库
			if err := s.Flush(); err != nil {
				s.logger.Error("insert data failed", zap.Error(err))
			}
		}
		s.dataDocker = append(s.dataDocker, cell)
	}
	return nil
}

// 用于将dataDocker中的数据单元批量插入数据库：遍历数据单元，将数据转换为合适的格式，并添加到args切片中，最后调用db的Insert方法将数据插入数据库
func (s *SqlStore) Flush() error {
	if len(s.dataDocker) == 0 {
		return nil
	}
	// Flush函数会将s.dataDocker中的数据单元批量插入数据库，当数据插入完成后，清空s.dataDocker
	defer func() {
		s.dataDocker = nil
	}()
	args := make([]interface{}, 0)
	for _, datacell := range s.dataDocker {
		ruleName := datacell.Data["Rule"].(string)
		taskName := datacell.Data["Task"].(string)
		fields := engine.GetFields(taskName, ruleName)
		data := datacell.Data["Data"].(map[string]interface{})
		// 声明一个字符串切片并初始化，后续会把从datacell.Data中提取出来的数据元素添加到这个切片中，即value切片会存储每一条数据记录的各个字段值，最后添加到args切片中
		value := []string{}
		for _, field := range fields {
			v := data[field]
			switch v.(type) {
			case nil:
				value = append(value, "")
			case string:
				value = append(value, v.(string))
			default:
				j, err := json.Marshal(v)
				if err != nil {
					value = append(value, "")
				} else {
					value = append(value, string(j))
				}
			}
		}
		value = append(value, datacell.Data["Url"].(string), datacell.Data["Time"].(string))
		for _, v := range value {
			args = append(args, v)
		}
	}

	return s.db.Insert(sqldb.TableData{
		TableName:   s.dataDocker[0].GetTableName(),
		ColumnNames: getFields(s.dataDocker[0]),
		Args:        args,
		DataCount:   len(s.dataDocker),
	})
}

// 用于获取表的列名信息，从数据单元中获取任务名和规则名，然后获取字段列表并将每个字段转换为sqldb.Field结构体，并添加到columnNames切片中，最后再添加url和time字段
func getFields(cell *collector.DataCell) []sqldb.Field {
	taskName := cell.Data["Task"].(string)
	ruleName := cell.Data["Rule"].(string)
	fields := engine.GetFields(taskName, ruleName)

	var columnNames []sqldb.Field
	for _, field := range fields {
		columnNames = append(columnNames, sqldb.Field{
			Title: field,
			Type:  "MEDIUMTEXT",
		})
	}
	columnNames = append(columnNames,
		sqldb.Field{Title: "Url", Type: "VARCHAR(255)"},
		sqldb.Field{Title: "Time", Type: "VARCHAR(255)"},
	)
	return columnNames
}
