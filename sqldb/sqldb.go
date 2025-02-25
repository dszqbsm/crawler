package sqldb

// 定义了用于与MySQL数据库进行交互的功能，包括创建表、插入数据

import (
	"database/sql"
	"errors"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
)

// 为数据库操作统一了规范，包括创建表、插入数据
type DBer interface {
	CreateTable(t TableData) error
	Insert(t TableData) error
}

// sql数据库实例
type Sqldb struct {
	options
	db *sql.DB
}

// 打开一个MySQL数据库连接，设置最大连接数和最大空闲连接数，通过ping方法测试连接是否正常
func (d *Sqldb) OpenDB() error {
	db, err := sql.Open("mysql", d.sqlUrl)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(2048)
	db.SetMaxIdleConns(2048)
	if err = db.Ping(); err != nil {
		return err
	}
	d.db = db
	return nil
}

// 定义了创建表的方法，根据TableData中的数据创建数据库表
func (d *Sqldb) CreateTable(t TableData) error {
	if len(t.ColumnNames) == 0 {
		return errors.New("Column can not be empty")
	}
	sql := `CREATE TABLE IF NOT EXISTS ` + t.TableName + " ("
	if t.AutoKey {
		sql += `id INT(12) NOT NULL PRIMARY KEY AUTO_INCREMENT,`
	}
	for _, t := range t.ColumnNames {
		sql += t.Title + ` ` + t.Type + `,`
	}
	sql = sql[:len(sql)-1] + `) ENGINE=MyISAM DEFAULT CHARSET=utf8;`

	d.logger.Debug("crate table", zap.String("sql", sql))

	_, err := d.db.Exec(sql)
	return err
}

// 定义了删除表的方法，根据TableData中的数据删除数据库表
func (d *Sqldb) DropTable(t TableData) error {
	if len(t.ColumnNames) == 0 {
		return errors.New("column can not be empty")
	}

	sql := `DROP TABLE ` + t.TableName

	d.logger.Debug("drop table", zap.String("sql", sql))

	_, err := d.db.Exec(sql)

	return err
}

// 定义了插入数据的方法，构造插入语句并执行插入操作，形如INSERT INTO users(id,name,age) VALUES (?,?,?),(?,?,?);，多少个问号取决于有多少列
func (d *Sqldb) Insert(t TableData) error {
	if len(t.ColumnNames) == 0 {
		return errors.New("empty column")
	}
	sql := `INSERT INTO ` + t.TableName + `(` // 初始化一个sql插入语句的前缀

	for _, v := range t.ColumnNames { // 遍历ColumnNames切片，将每个字段的标题添加到sql插入语句中，列名之间用逗号隔开
		sql += v.Title + ","
	}

	sql = sql[:len(sql)-1] + `) VALUES ` // 去掉最后一个多余都好，并添加values关键字

	blank := ",(" + strings.Repeat(",?", len(t.ColumnNames))[1:] + ")" // 构建一个占位符，由逗号和问号组成，问号数量等于列的数量
	sql += strings.Repeat(blank, t.DataCount)[1:] + `;`                // 去除多余逗号并添加分号，完成sql插入语句的构建
	d.logger.Debug("insert table", zap.String("sql", sql))
	_, err := d.db.Exec(sql, t.Args...)
	return err
}

// 表示数据库表中的一个字段，包含字段名和字段类型
type Field struct {
	Title string
	Type  string
}

// 表示要操作的数据库表的数据
type TableData struct {
	TableName   string
	ColumnNames []Field       // 标题字段
	Args        []interface{} // 数据
	DataCount   int           // 插入数据的数量
	AutoKey     bool
}

// 创建一个新的Sqldb实例，并根据传入的选项进行配置
func New(opts ...Option) (*Sqldb, error) {
	options := defaultOptions
	for _, opt := range opts {
		opt(&options)
	}
	d := &Sqldb{}
	d.options = options
	if err := d.OpenDB(); err != nil {
		return nil, err
	}
	return d, nil
}
