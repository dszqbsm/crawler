// 提供默认配置，包括日志编码器、日志选项和文件日志的轮转配置，被log.go调用，用于初始化日志插件和日志器

package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// 默认的一些配置

func DefaultEncoderConfig() zapcore.EncoderConfig {
	var encoderConfig = zap.NewProductionEncoderConfig()
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	return encoderConfig
}

// 统一用json
func DefaultEncoder() zapcore.Encoder {
	return zapcore.NewJSONEncoder(DefaultEncoderConfig())
}

// 默认会添加调用者的信息和堆栈跟踪
// 默认的选项会输出调用时的文件与行号，并且只有当日志等级在DPanic以上时，才输出函数的堆栈信息
func DefaultOption() []zap.Option {
	var stackTraceLevel zap.LevelEnablerFunc = func(level zapcore.Level) bool {
		return level >= zapcore.DPanicLevel
	}
	return []zap.Option{
		zap.AddCaller(),
		zap.AddStacktrace(stackTraceLevel),
	}
}

// 1.不会自动清理backup
// 2.每200mb压缩一次，不按时间rotate
// 日志文件最大为200MB，超过后会自动轮转并压缩文件
func DefaultLumberjackLogger() *lumberjack.Logger {
	return &lumberjack.Logger{
		MaxSize:   200,
		LocalTime: true,
		Compress:  true,
	}
}
