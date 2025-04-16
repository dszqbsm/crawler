package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

/*
无输入，输出一个Zap日志库的编码器配置

获取生产环境默认编码器配置后，修改日志级别的格式化方式为大写，修改时间的格式化方式为ISO8601格式
*/
func DefaultEncoderConfig() zapcore.EncoderConfig {
	var encoderConfig = zap.NewProductionEncoderConfig()
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	return encoderConfig
}

/*
无输入，输出一个Zap日志库的编码器

使用DefaultEncoderConfig()获取自定义的编码器配置，生成Zap日志库的编码器
*/
func DefaultEncoder() zapcore.Encoder {
	return zapcore.NewJSONEncoder(DefaultEncoderConfig())
}

/*
无输入，输出一个Zap日志库的选项列表

定义堆栈跟踪的出发级别为仅当日志级别>=DpanicLevel时才记录堆栈跟踪信息，最后在返回的配置选项切片中添加调用者信息（文件名和行号），并设置按条件记录堆栈跟踪信息
*/
func DefaultOption() []zap.Option {
	var stackTraceLevel zap.LevelEnablerFunc = func(level zapcore.Level) bool {
		return level >= zapcore.DPanicLevel
	}
	return []zap.Option{
		zap.AddCaller(),
		zap.AddStacktrace(stackTraceLevel),
	}
}

/*
无输入，输出一个预配置的日志轮转器

基于lumberjack库的默认配置，设置日志文件的最大大小为200MB，启用本地时间记录，启用压缩，返回预配置的日志轮转器
*/
func DefaultLumberjackLogger() *lumberjack.Logger {
	return &lumberjack.Logger{
		MaxSize:   200,
		LocalTime: true,
		Compress:  true,
	}
}
