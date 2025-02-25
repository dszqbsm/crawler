package main

import (
	"github.com/dszqbsm/crawler/cmd"
)

func main() {
	cmd.Execute()
}

/* import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/dszqbsm/crawler/collect"
	"github.com/dszqbsm/crawler/collector/sqlstorage"
	"github.com/dszqbsm/crawler/engine"
	"github.com/dszqbsm/crawler/limiter"
	"github.com/dszqbsm/crawler/log"
	pb "github.com/dszqbsm/crawler/proto/greeter"
	etcdReg "github.com/go-micro/plugins/v4/registry/etcd"
	gs "github.com/go-micro/plugins/v4/server/grpc"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go-micro.dev/v4"
	"go-micro.dev/v4/registry"
	"go-micro.dev/v4/server"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
)

func main() {
	// log
	plugin := log.NewStdoutPlugin(zapcore.InfoLevel)
	logger := log.NewLogger(plugin)
	logger.Info("log init end")

	zap.ReplaceGlobals(logger)

	var f collect.Fetcher = &collect.BrowserFetch{
		Timeout: 3000 * time.Millisecond,
		Logger:  logger,
		Proxy:   nil,
	}

	// var storage collector.Storage
	storage, err := sqlstorage.New(
		sqlstorage.WithSqlUrl("root:123456@tcp(127.0.0.1:3326)/crawler?charset=utf8"),
		sqlstorage.WithLogger(logger.Named("sqlDB")),
		sqlstorage.WithBatchCount(2),
	)
	if err != nil {
		logger.Error("create sqlstorage failed")
		return
	}

	//2秒钟1个
	secondLimit := rate.NewLimiter(limiter.Per(1, 2*time.Second), 1)
	//60秒20个
	minuteLimit := rate.NewLimiter(limiter.Per(20, 1*time.Minute), 20)
	multiLimiter := limiter.MultiLimiter(secondLimit, minuteLimit)

	seeds := make([]*collect.Task, 0, 1000)
	seeds = append(seeds, &collect.Task{
		Property: collect.Property{
			Name: "douban_book_list",
		},
		Fetcher: f,
		Storage: storage,
		Limit:   multiLimiter,
	})

	s := engine.NewEngine(
		engine.WithFetcher(f),
		engine.WithLogger(logger),
		engine.WithWorkCount(5),
		engine.WithSeeds(seeds),
		engine.WithScheduler(engine.NewSchedule()),
	)

	// worker start
	go s.Run()

	// start http proxy to GRPC
	go HandleHTTP()

	// 将 HTTP 请求代理到 gRPC 服务，并启动一个 gRPC 服务器来处理 Greeter 服务的请求

	// 创建一个基于etcd的服务注册中心实例，并指定etcd服务的地址
	reg := etcdReg.NewRegistry(
		registry.Addrs(":2379"),
	)
	// 创建一个微服务实例
	service := micro.NewService(
		micro.Server(gs.NewServer( // 指定微服务使用的grpc服务器
			server.Id("1"), // 用于指定服务器的id
		)),
		micro.Address(":9090"),               // 指定微服务的监听地址
		micro.Registry(reg),                  // 指定微服务使用的注册中心
		micro.Name("go.micro.server.worker"), // 指定微服务的名称
	)
	service.Init()                                            // 初始化微服务实例
	pb.RegisterGreeterHandler(service.Server(), new(Greeter)) // 将Greeter服务注册到micro生成的grpc服务器上
	if err := service.Run(); err != nil {                     // 启动微服务实例
		logger.Fatal("grpc server stop")
	}
}

type Greeter struct{}

// 实现Greeter服务的Hello方法
func (g *Greeter) Hello(ctx context.Context, req *pb.Request, rsp *pb.Response) error {
	rsp.Greeting = "Hello " + req.Name
	return nil
}

// 用于将http请求代理到grpc服务，允许客户端通过http请求调用grpc服务
// 该函数生成http服务器，监听8080端口，利用grpc-gateway生成了RegisterGreeterGwFromEndpoint方法，指定了要转发到哪一个grpc服务器
func HandleHTTP() {
	ctx := context.Background()            // 创建一个空的context
	ctx, cancel := context.WithCancel(ctx) // 创建一个可以取消的context
	defer cancel()

	mux := runtime.NewServeMux()                   // 创建一个新的http多路复用器
	opts := []grpc.DialOption{grpc.WithInsecure()} // 创建一个grpc连接选项，用于连接到grpc服务器

	err := pb.RegisterGreeterGwFromEndpoint(ctx, mux, "localhost:9090", opts) // 将Greeter服务的grpc网关注册到http多路复用器mux上
	if err != nil {
		fmt.Println(err)
	}
	// 启动一个http服务器，监听8080端口，并将请求转发到mux
	http.ListenAndServe(":8080", mux)
} */
