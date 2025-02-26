package worker

import (
	"context"
	"net/http"
	"time"

	"github.com/dszqbsm/crawler/collect"
	"github.com/dszqbsm/crawler/engine"
	"github.com/dszqbsm/crawler/limiter"
	"github.com/dszqbsm/crawler/log"
	"github.com/dszqbsm/crawler/proto/greeter"
	"github.com/dszqbsm/crawler/proxy"
	"github.com/dszqbsm/crawler/spider"
	"github.com/dszqbsm/crawler/storage/sqlstorage"
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
		&workerID, "id", "1", "set master id")
	WorkerCmd.Flags().StringVar(
		&HTTPListenAddress, "http", ":8080", "set HTTP listen address")

	WorkerCmd.Flags().StringVar(
		&GRPCListenAddress, "grpc", ":9090", "set GRPC listen address")
}

var workerID string
var HTTPListenAddress string
var GRPCListenAddress string

func Run() {
	var (
		err     error
		logger  *zap.Logger
		p       proxy.ProxyFunc
		storage spider.Storage
	)

	// 通过toml加载配置
	enc := toml.NewEncoder()                                                                 // 创建一个toml编码器，用于编码和解码toml格式的数据
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
	var f spider.Fetcher = &collect.BrowserFetch{
		Timeout: time.Duration(timeout) * time.Millisecond,
		Logger:  logger,
		Proxy:   p,
	}

	// 存储器配置
	sqlURL := cfg.Get("storage", "sqlURL").String("")
	if storage, err = sqlstorage.New(
		sqlstorage.WithSqlUrl(sqlURL),
		sqlstorage.WithLogger(logger.Named("sqlDB")),
		sqlstorage.WithBatchCount(2),
	); err != nil {
		logger.Error("create sqlstorage failed", zap.Error(err))
		return
	}

	// 初始化任务
	var tcfg []spider.TaskConfig
	if err := cfg.Get("Tasks").Scan(&tcfg); err != nil {
		logger.Error("init seed tasks", zap.Error(err))
	}
	// 解析任务配置
	seeds := ParseTaskConfig(logger, f, storage, tcfg)

	// 初始化爬虫引擎环境
	_ = engine.NewEngine(
		engine.WithFetcher(f),
		engine.WithLogger(logger),
		engine.WithWorkCount(5),
		engine.WithSeeds(seeds),
		engine.WithScheduler(engine.NewSchedule()),
	)

	// 启动worker节点
	// go s.Run()

	var sconfig ServerConfig
	if err := cfg.Get("GRPCServer").Scan(&sconfig); err != nil {
		logger.Error("get GRPC Server config failed", zap.Error(err))
	}
	logger.Sugar().Debugf("grpc server config,%+v", sconfig)

	// 启动HTTP代理服务器，将HTTP请求转发到grpc服务器
	go RunHTTPServer(sconfig)

	// 启动grpc服务器
	RunGRPCServer(logger, sconfig)
}

type ServerConfig struct {
	// GRPCListenAddress string
	// HTTPListenAddress string
	// ID                string
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

// 将配置文件中的任务配置解析为可执行的任务实例，为每个任务设置必要的属性，最终返回任务切片，包含了所有的任务
func ParseTaskConfig(logger *zap.Logger, f spider.Fetcher, s spider.Storage, cfgs []spider.TaskConfig) []*spider.Task {
	tasks := make([]*spider.Task, 0, 1000)
	// 对于每一个任务都创建一个爬虫实例
	for _, cfg := range cfgs {
		t := spider.NewTask(
			spider.WithName(cfg.Name),
			spider.WithReload(cfg.Reload),
			spider.WithCookie(cfg.Cookie),
			spider.WithLogger(logger),
			spider.WithStorage(s),
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
				// speed limiter
				l := rate.NewLimiter(limiter.Per(lcfg.EventCount, time.Duration(lcfg.EventDur)*time.Second), 1)
				limits = append(limits, l)
			}
			multiLimiter := limiter.Multi(limits...)
			t.Limit = multiLimiter
		}

		switch cfg.Fetcher {
		case "browser":
			t.Fetcher = f
		}
		tasks = append(tasks, t)
	}
	return tasks
}
