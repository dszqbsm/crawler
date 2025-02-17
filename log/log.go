// 日志功能的核心实现，定义了日志插件和日志器的创建逻辑，以来default.go中的默认配置

package log

import (
	"io"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Plugin = zapcore.Core

// NOTE: 一些option选项是无法覆盖的
// 用来创建一个新的zap.Logger实例
func NewLogger(plugin zapcore.Core, options ...zap.Option) *zap.Logger {
	return zap.New(plugin, append(DefaultOption(), options...)...)
}

// 创建一个新的日志插件，接收一个日志输出目标和一个日志级别过滤器，使用默认编码配置
func NewPlugin(writer zapcore.WriteSyncer, enabler zapcore.LevelEnabler) Plugin {
	return zapcore.NewCore(DefaultEncoder(), writer, enabler)
}

// 创建输出到标准输出的日志插件
func NewStdoutPlugin(enabler zapcore.LevelEnabler) Plugin {
	return NewPlugin(zapcore.Lock(zapcore.AddSync(os.Stdout)), enabler)
}

// 创建输出到标准错误的日志插件
func NewStderrPlugin(enabler zapcore.LevelEnabler) Plugin {
	return NewPlugin(zapcore.Lock(zapcore.AddSync(os.Stderr)), enabler)
}

// Lumberjack logger虽然持有File但没有暴露sync方法，所以没办法利用zap的sync特性
// 所以额外返回一个closer，需要保证在进程退出前close以保证写入的内容可以全部刷到到磁盘
// 创建输出到文件的日志插件，使用lumberjack.Logger实现日志文件的轮转和压缩
func NewFilePlugin(
	filePath string, enabler zapcore.LevelEnabler) (Plugin, io.Closer) {
	var writer = DefaultLumberjackLogger()
	writer.Filename = filePath
	return NewPlugin(zapcore.AddSync(writer), enabler), writer
}
