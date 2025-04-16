package master

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/dszqbsm/crawler/generator"
	"github.com/dszqbsm/crawler/log"
	"github.com/dszqbsm/crawler/master"
	proto "github.com/dszqbsm/crawler/proto/crawler"
	grpccli "github.com/go-micro/plugins/v4/client/grpc"
	"github.com/go-micro/plugins/v4/config/encoder/toml"
	"github.com/go-micro/plugins/v4/registry/etcd"
	"github.com/go-micro/plugins/v4/server/grpc"
	"github.com/go-micro/plugins/v4/wrapper/breaker/hystrix"
	ratePlugin "github.com/go-micro/plugins/v4/wrapper/ratelimiter/ratelimit"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/juju/ratelimit"
	"github.com/spf13/cobra"
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

/*
命令提示和输入命令后会调用的执行函数
*/
var MasterCmd = &cobra.Command{
	Use:   "master",
	Short: "run master service.",
	Long:  "run master service.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		Run()
	},
}

// init函数会在包被初始化时（包的初始化是在程序启动时自动进行的，即当一个包被导入时，go会先初始化该包，如A导入包B，则B会被先初始化，再初始化包A）自动执行，不需要手动调用，用于执行必要的初始化操作，初始化包级变量
// 在init函数中为mastercmd命令添加了三个命令行标志，允许用户在启动master服务时指定不同的配置项，如masterID、HTTPListenAddress和GRPCListenAddress，默认是1、8081、9091
func init() {
	MasterCmd.Flags().StringVar(
		&masterID, "id", "1", "set master id")
	MasterCmd.Flags().StringVar(
		&HTTPListenAddress, "http", ":8081", "set HTTP listen address")
	MasterCmd.Flags().StringVar(
		&GRPCListenAddress, "grpc", ":9091", "set GRPC listen address")
	MasterCmd.Flags().StringVar(
		&cfgFile, "config", "config.toml", "set master id")
	MasterCmd.Flags().StringVar(
		&podIP, "podip", "", "set worker id")
	MasterCmd.Flags().StringVar(
		&PProfListenAddress, "pprof", ":9981", "set pprof address")
}

// 全局变量，用于存储从命令行标志获取的值
var masterID string
var HTTPListenAddress string
var GRPCListenAddress string
var cfgFile string
var PProfListenAddress string
var podIP string

/*
无输入，无输出

该函数作为master服务的入口，用于启动master服务，主要包括以下步骤：

1、加载toml配置文件，初始化日志记录器，创建etcd注册中心实例，创建master实例

2、启动HTTP代理服务器，将HTTP请求转发到grpc服务器

3、启动grpc服务器，处理grpc请求
*/
func Run() {
	// start pprof
	// 用于启用性能分析端点，主要用于开发调试阶段的性能监控和问题排查
	go func() {
		if err := http.ListenAndServe(PProfListenAddress, nil); err != nil {
			panic(err)
		}
	}()
	var (
		err    error
		logger *zap.Logger
	)

	// 通过toml加载配置
	enc := toml.NewEncoder()                                                                 // 创建一个toml编码器，用于编码和解码toml格式的数据
	cfg, err := config.NewConfig(config.WithReader(json.NewReader(reader.WithEncoder(enc)))) // 创建一个配置实例cfg，并指定使用json读取器和toml编码器
	err = cfg.Load(file.NewSource(                                                           // 调用Load方法从config.toml文件中加载配置信息
		file.WithPath(cfgFile),
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

	var sconfig ServerConfig
	if err := cfg.Get("MasterServer").Scan(&sconfig); err != nil { // 从配置中获取MasterServer配置项，并将其扫描到sconfig变量中
		logger.Error("get GRPC Server config failed", zap.Error(err))
	}
	logger.Sugar().Debugf("grpc server config,%+v", sconfig)

	reg := etcd.NewRegistry(registry.Addrs(sconfig.RegistryAddress)) // 创建一个基于etcd的服务注册中心实例，并指定etcd的地址

	/*
		需要补充加载本地种子任务的逻辑
	*/

	// 创建master实例，初始化服务环境
	m, err := master.New(
		masterID,
		master.WithLogger(logger.Named("master")),
		master.WithGRPCAddress(GRPCListenAddress),
		master.WithregistryURL(sconfig.RegistryAddress),
		master.WithRegistry(reg),
	)
	if err != nil {
		logger.Error("init  master falied", zap.Error(err))
	}

	// 启动HTTP代理服务器，将HTTP请求转发到grpc服务器
	go RunHTTPServer(sconfig)

	// 启动grpc服务器
	RunGRPCServer(m, logger, reg, sconfig)
}

type ServerConfig struct {
	RegistryAddress  string // 注册中心地址
	RegisterTTL      int    // 注册中心注册TTL
	RegisterInterval int    // 注册中心注册间隔
	Name             string // 服务名称
	ClientTimeOut    int    // 客户端超时时间
}

/*
输入一个master实例、日志器、etcd注册中心实例、master配置结构体，无输出

该函数用于创建master节点微服务实例，主要包括以下步骤：

1、基于go-micro框架创建一个微服务实例service，配置服务id、grpc监听地址、注册中心、注册时间间隔、限速器、日志包装器、服务名称、客户端配置、客户端熔断
*/
func RunGRPCServer(m *master.Master, logger *zap.Logger, reg registry.Registry, cfg ServerConfig) {
	// 创建令牌桶限流器（0.5 token/秒，桶容量1）
	b := ratelimit.NewBucketWithRate(0.5, 1)
	// Master ID生成逻辑
	if masterID == "" {
		if podIP != "" { // 优先使用Pod IP生成ID
			ip := generator.IDbyIP(podIP)
			masterID = strconv.Itoa(int(ip))
		} else { // 无Pod IP时使用时间戳
			masterID = fmt.Sprintf("%d", time.Now().UnixNano())
		}
	}
	zap.S().Debug("master id:", masterID)
	// 创建微服务实例（核心容器）
	service := micro.NewService(
		// 配置gRPC服务器（携带唯一ID）
		micro.Server(grpc.NewServer(
			server.Id(masterID),
		)),
		// 设置监听地址
		micro.Address(GRPCListenAddress),
		// 注入服务注册中心
		micro.Registry(reg),
		// 设置注册中心TTL（存活时间）
		micro.RegisterTTL(time.Duration(cfg.RegisterTTL)*time.Second),
		// 设置注册中心注册间隔时间
		micro.RegisterInterval(time.Duration(cfg.RegisterInterval)*time.Second),
		// 服务端限流中间件（令牌桶实现）
		micro.WrapHandler(ratePlugin.NewHandlerWrapper(b, false)),
		// 添加日志中间件
		micro.WrapHandler(logWrapper(logger)),
		// 设置服务名称
		micro.Name(cfg.Name),
		// 使用gRPC协议客户端
		micro.Client(grpccli.NewClient()),
		// 客户端熔断中间件（Hystrix）
		micro.WrapClient(hystrix.NewClientWrapper()),
	)
	// 即master服务中还包含了grpc客户端，通过grpc客户端调用worker节点的grpc服务，因此需要给grpc客户端设置熔断机制，防止下游worker节点故障导致级联故障

	// master是微服务？grpc服务端？grpc客户端？
	// master实例是什么，定义的service又是什么，grpc服务端、grpc客户端又有什么作用，架构上没理解清楚

	// 配置熔断策略
	hystrix.ConfigureCommand("go.micro.server.master.CrawlerMaster.AddResource", hystrix.CommandConfig{
		Timeout:                10000, // 10秒超时
		MaxConcurrentRequests:  100,   // 最大并发量
		RequestVolumeThreshold: 10,    // 10个请求样本
		SleepWindow:            6000,  // 6秒熔断冷却期
		ErrorPercentThreshold:  30,    // 30%错误率触发熔断
	})

	// 创建服务客户端实例（用于调用自身服务）
	cl := proto.NewCrawlerMasterService(cfg.Name, service.Client())
	// 将客户端注入主节点
	m.SetForwardCli(cl)

	// 初始化微服务客户端，设置请求超时时间
	if err := service.Client().Init(client.RequestTimeout(time.Duration(cfg.ClientTimeOut) * time.Second)); err != nil {
		logger.Sugar().Error("micro client init error. ", zap.String("error:", err.Error()))
		return
	}
	// 初始化服务实例，会将服务注册到注册中心，并能启动后台协程维护心跳
	service.Init()

	// 注册gRPC处理器（将Master与gRPC服务绑定），将业务实现m与服务绑定，将master服务实现注册到grpc服务器，使其他节点可以通过grpc调用master服务接口
	if err := proto.RegisterCrawlerMasterHandler(service.Server(), m); err != nil {
		logger.Fatal("register handler failed", zap.Error(err))
	}

	// 启动微服务，监听并处理客户端的grpc请求
	if err := service.Run(); err != nil {
		logger.Fatal("grpc server stop")
	}
}

/*
输入一个ServerConfig结构体，无输出

该函数主要用于实现一个HTTP反向代理服务器，将HTTP请求转换为gRPC请求，实现REST ful API和gRPC服务的双向通信

1、初始化一个gRPC-Gateway的请求路由器，用于将HTTP请求路径映射到gRPC方法

2、创建grpc连接选项，基于请求路由器，指定gRPC服务器地址，将master的gRPC服务注册到请求路由器中，实现HTTP和gRPC的互通

3、启动HTTP服务开启监听，将HTTP请求转发到gRPC服务器，实现HTTP和gRPC的互通
*/
func RunHTTPServer(cfg ServerConfig) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx) // 创建一个可以取消的context
	defer cancel()

	mux := runtime.NewServeMux() // 创建一个新的http多路复用器
	opts := []grpc2.DialOption{
		grpc2.WithTransportCredentials(insecure.NewCredentials()), // 创建一个grpc连接选项，用于连接到grpc服务器
	}

	if err := proto.RegisterCrawlerMasterGwFromEndpoint(ctx, mux, GRPCListenAddress, opts); err != nil {
		zap.L().Fatal("Register backend grpc server endpoint failed", zap.Error(err))
	}
	zap.S().Debugf("start master http server listening on %v proxy to grpc server;%v", HTTPListenAddress, GRPCListenAddress)
	if err := http.ListenAndServe(HTTPListenAddress, mux); err != nil {
		zap.L().Fatal("http listenAndServe failed", zap.Error(err))
	}
}

// 用于包装grpc服务器的处理程序，实现日志记录功能，即在处理grpc请求时记录请求信息和响应信息
func logWrapper(log *zap.Logger) server.HandlerWrapper {
	return func(fn server.HandlerFunc) server.HandlerFunc {
		return func(ctx context.Context, req server.Request, rsp interface{}) error {

			log.Info("receive request",
				zap.String("method", req.Method()),
				zap.String("Service", req.Service()),
				zap.String("Endpoint", req.Endpoint()),
				zap.Reflect("request param:", req.Body()),
			)

			err := fn(ctx, req, rsp)

			return err
		}
	}
}
