package worker

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/dszqbsm/crawler/generator"
	"github.com/dszqbsm/crawler/limiter"
	"github.com/dszqbsm/crawler/log"
	"github.com/dszqbsm/crawler/proto/greeter"
	"github.com/dszqbsm/crawler/proxy"
	"github.com/dszqbsm/crawler/spider"
	engine "github.com/dszqbsm/crawler/spider/workerengine"
	sqlstorage "github.com/dszqbsm/crawler/sqlstorage"
	"github.com/go-micro/plugins/v4/config/encoder/toml"
	"github.com/go-micro/plugins/v4/registry/etcd"
	"github.com/go-micro/plugins/v4/server/grpc"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
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
	"golang.org/x/time/rate"
	grpc2 "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var ServiceName string = "go.micro.server.worker"

var WorkerCmd = &cobra.Command{
	Use:   "worker",
	Short: "run worker service.",
	Long:  "run worker service.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		Run()
	},
}

func init() {
	WorkerCmd.Flags().StringVar(
		&workerID, "id", "", "set worker id")
	WorkerCmd.Flags().StringVar(
		&podIP, "podip", "", "set worker id")
	WorkerCmd.Flags().StringVar(
		&HTTPListenAddress, "http", ":8080", "set HTTP listen address")
	WorkerCmd.Flags().StringVar(
		&PProfListenAddress, "pprof", ":9981", "set pprof address")
	WorkerCmd.Flags().BoolVar(
		&cluster, "cluster", true, "run mode")
	WorkerCmd.Flags().StringVar(
		&GRPCListenAddress, "grpc", ":9090", "set GRPC listen address")
}

var workerID string
var HTTPListenAddress string
var GRPCListenAddress string
var cluster bool
var PProfListenAddress string
var podIP string

/*
无输入，无输出

该函数作为worker服务的入口，用于启动worker服务，主要包括以下步骤：

1、启动性能分析端点

2、加载toml配置文件，初始化日志器、采集器、存储器等组件，从配置文件中读取解析配置，初始化任务列表，生成workerid

4、创建爬虫引擎，配置采集器、日志器、worker节点数量、任务列表、调度器、数据存储器、workerid、请求历史存储器、资源存储器、资源注册器（etcd）

5、启动爬虫引擎、启动HTTP服务器、启动GRPC服务器
*/
func Run() {
	// 用于启动性能分析端点
	go func() {
		if err := http.ListenAndServe(PProfListenAddress, nil); err != nil {
			panic(err)
		}
	}()
	var (
		err     error
		logger  *zap.Logger
		p       proxy.ProxyFunc
		storage spider.DataRepository
	)

	// 通过toml加载配置
	enc := toml.NewEncoder() // 创建一个toml编码器，用于编码和解码toml格式的数据
	// cfg字段存储了toml文件中的所有配置
	cfg, err := config.NewConfig(config.WithReader(json.NewReader(reader.WithEncoder(enc)))) // 创建一个配置实例cfg，并指定使用json读取器和toml编码器
	err = cfg.Load(file.NewSource(                                                           // 调用Load方法从config.toml文件中加载配置信息
		file.WithPath("config.toml"),
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

	// 采集器配置
	proxyURLs := cfg.Get("fetcher", "proxy").StringSlice([]string{})
	timeout := cfg.Get("fetcher", "timeout").Int(5000)
	logger.Sugar().Info("proxy list: ", proxyURLs, " timeout: ", timeout)
	if p, err = proxy.RoundRobinProxySwitcher(proxyURLs...); err != nil {
		logger.Error("RoundRobinProxySwitcher failed", zap.Error(err))
	}
	f := spider.NewFetchService(spider.BrowserFetchType)

	// 存储器配置
	sqlURL := cfg.Get("storage", "sqlURL").String("")
	if storage, err = sqlstorage.New(
		sqlstorage.WithSqlUrl(sqlURL),
		sqlstorage.WithLogger(logger.Named("sqlDB")),
		sqlstorage.WithBatchCount(2),
	); err != nil {
		logger.Error("create sqlstorage failed", zap.Error(err))
		panic(err)
	}

	// 初始化任务
	var tcfg []spider.TaskConfig
	if err := cfg.Get("Tasks").Scan(&tcfg); err != nil {
		logger.Error("init seed tasks", zap.Error(err))
	}
	// 解析任务配置
	seeds := ParseTaskConfig(logger, p, f, storage, tcfg)

	var sconfig ServerConfig
	if err := cfg.Get("GRPCServer").Scan(&sconfig); err != nil {
		logger.Error("get GRPC Server config failed", zap.Error(err))
	}
	logger.Sugar().Debugf("grpc server config,%+v", sconfig)

	// 生成workerid的优先级逻辑，在k8s环境下使用容器ip生成id，非容器化环境使用时间戳生成id
	if workerID == "" {
		if podIP != "" {
			ip := generator.IDbyIP(podIP)
			workerID = strconv.Itoa(int(ip))
		} else {
			workerID = fmt.Sprintf("%d", time.Now().UnixNano())
		}
	}

	id := sconfig.Name + "-" + workerID
	zap.S().Debug("worker id:", id)

	reg, err := spider.NewEtcdRegistry([]string{sconfig.RegistryAddress})
	if err != nil {
		logger.Error("init EtcdRegistry failed", zap.Error(err))
	}

	s, err := engine.NewWorkerService(
		engine.WithFetcher(f),
		engine.WithLogger(logger),
		engine.WithWorkCount(5),
		engine.WithSeeds(seeds),
		// 实现服务发现，worker节点自动注册到etcd，从而master可动态发现所有worker
		//engine.WithregistryURL(sconfig.RegistryAddress),
		engine.WithScheduler(engine.NewSchedule()),
		// 实现数据持久化
		engine.WithStorage(storage),
		engine.WithID(id),
		engine.WithReqRepository(spider.NewReqHistoryRepository()),
		engine.WithResourceRepository(spider.NewResourceRepository()),
		engine.WithResourceRegistry(reg),
	)

	if err != nil {
		logger.Error("create worker service failed", zap.Error(err))
	}

	go s.Run(cluster)

	// 启动HTTP代理服务器，将HTTP请求转发到grpc服务器
	go RunHTTPServer(sconfig)

	// 启动grpc服务器
	RunGRPCServer(logger, sconfig)
}

type ServerConfig struct {
	RegistryAddress  string
	RegisterTTL      int
	RegisterInterval int
	Name             string
	ClientTimeOut    int
}

func RunGRPCServer(logger *zap.Logger, cfg ServerConfig) {
	reg := etcd.NewRegistry(registry.Addrs(cfg.RegistryAddress))
	service := micro.NewService(
		micro.Server(grpc.NewServer(
			server.Id(workerID),
		)),
		micro.Address(GRPCListenAddress),
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

	service.Init()

	if err := greeter.RegisterGreeterHandler(service.Server(), new(Greeter)); err != nil {
		logger.Fatal("register handler failed")
	}

	if err := service.Run(); err != nil {
		logger.Fatal("grpc server stop")
	}
}

type Greeter struct{}

func (g *Greeter) Hello(ctx context.Context, req *greeter.Request, rsp *greeter.Response) error {
	rsp.Greeting = "Hello " + req.Name

	return nil
}

/*
输入一个ServerConfig结构体，无输出

该函数主要用于实现一个HTTP反向代理服务器，将HTTP请求转换为gRPC请求，实现REST ful API和gRPC服务的双向通信

1、初始化一个gRPC-Gateway的请求路由器mux，用于将HTTP请求路径映射到gRPC方法

2、创建grpc连接选项，基于请求路由器，指定gRPC服务器地址，将master的gRPC服务注册到请求路由器中，实现HTTP和gRPC的互通

3、启动HTTP服务开启监听，将HTTP请求转发到gRPC服务器，实现HTTP和gRPC的互通
*/
func RunHTTPServer(cfg ServerConfig) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	defer cancel()

	mux := runtime.NewServeMux()
	opts := []grpc2.DialOption{
		grpc2.WithTransportCredentials(insecure.NewCredentials()),
	}

	if err := greeter.RegisterGreeterGwFromEndpoint(ctx, mux, GRPCListenAddress, opts); err != nil {
		zap.L().Fatal("Register backend grpc server endpoint failed", zap.Error(err))
	}
	zap.S().Debugf("start http server listening on %v proxy to grpc server;%v", HTTPListenAddress, GRPCListenAddress)
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
				zap.Reflect("request param:", req.Body()),
			)

			err := fn(ctx, req, rsp)

			return err
		}
	}
}

// 将配置文件中的任务配置解析为任务配置结构体，为每个任务设置必要的属性，最终返回任务切片，包含了所有的任务
/*
输入一个日志器、代理服务器、采集器、存储器、任务配置切片，输出一个任务切片

该函数用于解析任务配置，将配置解析为爬虫任务列表

1、初始化一个空的任务切片tasks

2、遍历任务配置切片，为每个任务创建一个新的任务实例t，并设置任务的名称、是否允许重新加载、cookie、日志器、存储器、代理服务器等属性

3、根据任务配置中的等待时间和最大深度设置任务的等待时间和最大深度属性

4、如果任务配置中包含限速器配置，则为任务创建一个限速器，并将其设置为任务的限速器属性

5、将任务的采集器设置为输入的采集器

6、将任务添加到任务切片tasks中
*/
func ParseTaskConfig(logger *zap.Logger, p proxy.ProxyFunc, f spider.Fetcher, s spider.DataRepository, cfgs []spider.TaskConfig) []*spider.Task {
	tasks := make([]*spider.Task, 0, 1000)
	for _, cfg := range cfgs {
		t := spider.NewTask(
			spider.WithName(cfg.Name),
			spider.WithReload(cfg.Reload),
			spider.WithCookie(cfg.Cookie),
			spider.WithLogger(logger),
			spider.WithStorage(s),
			spider.WithProxy(p),
		)

		if cfg.WaitTime > 0 {
			t.WaitTime = cfg.WaitTime
		}

		if cfg.MaxDepth > 0 {
			t.MaxDepth = cfg.MaxDepth
		}

		var limits []limiter.RateLimiter
		if len(cfg.Limits) > 0 {
			for _, lcfg := range cfg.Limits {
				l := rate.NewLimiter(limiter.Per(lcfg.EventCount, time.Duration(lcfg.EventDur)*time.Second), 1)
				limits = append(limits, l)
			}
			multiLimiter := limiter.Multi(limits...)
			t.Limit = multiLimiter
		}

		t.Fetcher = f
		tasks = append(tasks, t)
	}
	return tasks
}
