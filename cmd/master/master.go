package master

import (
	"context"
	"net/http"
	"time"

	"github.com/dszqbsm/crawler/log"
	"github.com/dszqbsm/crawler/proto/greeter"
	"github.com/go-micro/plugins/v4/config/encoder/toml"
	"github.com/go-micro/plugins/v4/registry/etcd"
	"github.com/go-micro/plugins/v4/server/grpc"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go-micro.dev/v4"
	"go-micro.dev/v4/client"
	"go-micro.dev/v4/config"
	"go-micro.dev/v4/config/reader"
	"go-micro.dev/v4/config/reader/json"
	"go-micro.dev/v4/config/source"
	"go-micro.dev/v4/config/source/file"
	"go-micro.dev/v4/registry"
	"go-micro.dev/v4/server"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	grpc2 "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// master.go文件实现了一个基于grpc和http代理的服务端程序，主要功能包括加载配置、初始化日志、启动HTTP代理服务器和grpc服务器，提供了一个简单的Greeter服务

func Run() {
	var (
		err    error
		logger *zap.Logger
	)

	// 通过toml加载配置
	enc := toml.NewEncoder()                                                                 // 创建一个toml编码器，用于编码和解码toml格式的数据
	cfg, err := config.NewConfig(config.WithReader(json.NewReader(reader.WithEncoder(enc)))) // 创建一个配置实例cfg，并指定使用json读取器和toml编码器
	err = cfg.Load(file.NewSource(                                                           // 调用Load方法从config.toml文件中加载配置信息
		source.WithEncoder(enc),
	))

	if err != nil {
		panic(err)
	}

	// log
	logText := cfg.Get("logLevel").String("INFO") // 从配置中获取logLevel的值，若未找到则默认使用INFO级别
	logLevel, err := zapcore.ParseLevel(logText)  // 将日志级别字符串解析为zapcore.Level类型的变量logLevel
	if err != nil {
		panic(err)
	}
	plugin := log.NewStdoutPlugin(logLevel) // 创建一个标准输出日志插件
	logger = log.NewLogger(plugin)          // 创建一个日志记录器
	logger.Info("log init end")

	// set zap global logger
	zap.ReplaceGlobals(logger)

	// fmt.Println("hello master")
	// Server
	var sconfig ServerConfig
	if err := cfg.Get("MasterServer").Scan(&sconfig); err != nil { // 从配置中获取MasterServer配置项，并将其扫描到sconfig变量中
		logger.Error("get MasterServer config failed", zap.Error(err))
		logger.Error("get GRPC Server config failed", zap.Error(err))
	}
	logger.Sugar().Debugf("grpc server config,%+v", sconfig)

	// 启动HTTP代理服务器，将HTTP请求转发到grpc服务器
	go RunHTTPServer(sconfig)

	// 启动grpc服务器
	RunGRPCServer(logger, sconfig)
}

type ServerConfig struct {
	GRPCListenAddress string // grpc服务器监听地址
	HTTPListenAddress string // http代理服务器监听地址
	ID                string // 服务器ID
	RegistryAddress   string // 注册中心地址
	RegisterTTL       int    // 注册中心注册TTL
	RegisterInterval  int    // 注册中心注册间隔
	Name              string // 服务名称
	ClientTimeOut     int    // 客户端超时时间
}

// 启动grpc服务器
func RunGRPCServer(logger *zap.Logger, cfg ServerConfig) {
	reg := etcd.NewRegistry(registry.Addrs(cfg.RegistryAddress)) // 创建一个etcd注册中心并指定注册中心地址
	service := micro.NewService(                                 // 创建一个微服务实例，并配置服务器、监听地址、注册中心、注册时间间隔、日志包装器、服务名称等参数
		micro.Server(grpc.NewServer(
			server.Id(cfg.ID),
		)),
		micro.Address(cfg.GRPCListenAddress),
		micro.Registry(reg),
		micro.RegisterTTL(time.Duration(cfg.RegisterTTL)*time.Second),
		micro.RegisterInterval(time.Duration(cfg.RegisterInterval)*time.Second),
		micro.WrapHandler(logWrapper(logger)),
		micro.Name(cfg.Name),
	)

	// 设置micro 客户端默认超时时间为10秒钟
	if err := service.Client().Init(client.RequestTimeout(time.Duration(cfg.ClientTimeOut) * time.Second)); err != nil {
		logger.Sugar().Error("micro client init error. ", zap.String("error:", err.Error()))
		return
	}
	// 初始化微服务
	service.Init()

	// 调用RegisterGreeterHandler函数，将Greeter服务注册到微服务中
	if err := greeter.RegisterGreeterHandler(service.Server(), new(Greeter)); err != nil {
		logger.Fatal("register handler failed")
	}

	// 启动微服务，监听并处理客户端的grpc请求
	if err := service.Run(); err != nil {
		logger.Fatal("grpc server stop")
	}
}

// 定义Greeter结构体并实现Greeter服务中定义的方法
type Greeter struct{}

func (g *Greeter) Hello(ctx context.Context, req *greeter.Request, rsp *greeter.Response) error {
	rsp.Greeting = "Hello " + req.Name
	return nil
}

// 启动HTTP代理服务器，将HTTP请求转发到grpc服务器
func RunHTTPServer(cfg ServerConfig) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx) // 创建一个可以取消的context
	defer cancel()

	mux := runtime.NewServeMux() // 创建一个新的http多路复用器
	opts := []grpc2.DialOption{
		grpc2.WithTransportCredentials(insecure.NewCredentials()), // 创建一个grpc连接选项，用于连接到grpc服务器
	}

	// 将Greeter服务的grpc网关注册到http多路复用器mux上，指定grpc服务器的地址
	if err := greeter.RegisterGreeterGwFromEndpoint(ctx, mux, cfg.GRPCListenAddress, opts); err != nil {
		zap.L().Fatal("Register backend grpc server endpoint failed")
	}
	zap.S().Debugf("start master http server listening on %v proxy to grpc server;%v", cfg.HTTPListenAddress, cfg.GRPCListenAddress)
	if err := http.ListenAndServe(cfg.HTTPListenAddress, mux); err != nil {
		zap.L().Fatal("http listenAndServe failed")
	}
}

// 用于包装grpc服务器的处理程序，实现日志记录功能，即在处理grpc请求时记录请求信息和响应信息
func logWrapper(log *zap.Logger) server.HandlerWrapper {
	return func(fn server.HandlerFunc) server.HandlerFunc {
		return func(ctx context.Context, req server.Request, rsp interface{}) error {

			log.Info("receive request",
				zap.String("method", req.Method()),
				zap.String("Service", req.Service()),
				zap.Reflect("request param:", req.Body()),
			)

			err := fn(ctx, req, rsp)

			return err
		}
	}
}
