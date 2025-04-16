package log

import (
	"io"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Plugin = zapcore.Core

/*
输入一个zapcore.Core日志核心和可选的Zap配置选项，输出一个配置好的Zap日志库实例

优先应用DefaultOption()函数返回的默认配置选项，然后将传入的额外配置选项添加到默认配置选项中，最后使用zap.New()函数创建并返回Zap日志库实例
*/
func NewLogger(plugin zapcore.Core, options ...zap.Option) *zap.Logger {
	return zap.New(plugin, append(DefaultOption(), options...)...)
}

/*
输入一个日志写入目标和日志级别过滤器，输出一个zapcore.Core实例

使用zapcore.NewCore()函数创建一个新的zapcore.Core实例，该实例使用DefaultEncoder()函数返回的默认编码器，将日志写入目标和日志级别过滤器作为参数传入，最后返回创建的zapcore.Core实例
*/
func NewPlugin(writer zapcore.WriteSyncer, enabler zapcore.LevelEnabler) Plugin {
	return zapcore.NewCore(DefaultEncoder(), writer, enabler)
}

/*
输入一个日志级别过滤器，输出一个zapcore.Core实例

使用NewPlugin()函数创建一个新的zapcore.Core实例，返回一个绑定到标准输出的zapcore.Core实例，该实例的日志级别过滤器为传入的日志级别过滤器
*/
func NewStdoutPlugin(enabler zapcore.LevelEnabler) Plugin {
	return NewPlugin(zapcore.Lock(zapcore.AddSync(os.Stdout)), enabler)
}

/*
输入一个日志级别过滤器，输出一个zapcore.Core实例

使用NewPlugin()函数创建一个新的zapcore.Core实例，返回一个绑定到标准错误输出的zapcore.Core实例，该实例的日志级别过滤器为传入的日志级别过滤器
*/
func NewStderrPlugin(enabler zapcore.LevelEnabler) Plugin {
	return NewPlugin(zapcore.Lock(zapcore.AddSync(os.Stderr)), enabler)
}

// Lumberjack logger虽然持有File但没有暴露sync方法，所以没办法利用zap的sync特性，所以额外返回一个closer，需要保证在进程退出前close以保证写入的内容可以全部刷到到磁盘
/*
输入一个日志文件路径和日志级别过滤器，输出一个zapcore.Core实例和一个io.Closer实例

初始化轮转配置，创建绑定到文件的zapcore.Core实例，返回该实例和轮转配置的io.Closer实例
*/
func NewFilePlugin(filePath string, enabler zapcore.LevelEnabler) (Plugin, io.Closer) {
	var writer = DefaultLumberjackLogger()
	writer.Filename = filePath
	return NewPlugin(zapcore.AddSync(writer), enabler), writer
}
