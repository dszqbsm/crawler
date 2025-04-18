# 高性能、分布式爬虫项目



# 学习笔记

Go语言正则表达式默认使用PCRE标准，也支持POSIX标准
XPath定义了一种遍历XML文档中结点层次结构，并返回匹配元素的灵活方法
XML是一种可扩展是标记语言，标识结构化信息的一种规范
CSS是一种定义HTML文档中元素样式的语言，在CSS文件中可以定义一个或多个HTML中的标签的路径，并指定这些标签的样式，定义标签路径的方法被成为CSS选择器
Go标准库使用连接池来优化获取连接的过程，①对于请求结束后的连接，会根据是否有等待连接的协程，若有则唤醒等待的协程并使用该连接发起请求，若没有则会将该连接放入连接池中；②对于请求开始时，会先从连接池中查看是否有对应的空闲连接，若有则使用该连接发起请求，若没有则该协程会阻塞等待，会通过异步方式与服务器建立连接，在这个过程中若有连接正好空闲，则该协程会优先使用该空闲连接，提高使用效率



# 功能的前因后果

因为服务器传输的HTML文本可能存在多种编码形式，为了让请求网站的功能具有通用性，需要考虑编码问题，使用官方处理字符集的库实现编码的通用性

因为strings、bytes标准库对字符串的处理都比较常规，而我们需要对爬取的页面做复杂的处理，因此需要用更强大的文本处理方法——正则表达式，根据网页源代码内容信息特点，使用正则表达式获取需要的内容

因为正则表达式较为复杂且消耗CPU资源，而HTML是结构化的数据，因此可以使用XPath来更高效的查找HTML中的数据

虽然XPath语法能够提高检索效率，但是XPath并不是专门为HTML设计的，因此可以使用CSS选择器这种专门为HTML设计的方法

由于爬取的网站越来越复杂，服务器本身的反爬机制等原因，需要用到不同的爬取技术，如模拟浏览器访问、代理访问等，为了能够容易的切换不同的爬取方法，对功能进行组合、测试，使用接口对采集功能进行抽象

为了绕过反爬机制，通过为请求添加user-agent头，或者借助浏览器驱动协议/谷歌开发者工具协议远程与浏览器交互，这样能够获取到某些需要与开发者交互才能得到的数据

与Selenium这类的浏览器驱动协议不同的是，谷歌开发者工具协议直接通过web socket协议与浏览器暴露的api进行通信，因此能够更快操作浏览器

go标准库无法对日志进行分级，且包含大量反射影响性能，因此使用知名的第三方日志库组件Zap，其不使用反射，效率较高，且支持日志分级

爬取网站时通常会遇到一连串需要继续爬取的url，通过将爬虫任务拆分，设计任务调度算法，使得任务并行执行，且不遗漏任何信息

为了同时爬取多个爬虫任务，需要合理组织和调度爬虫任务，需要设计调度引擎对任务进行调度和组织

任务调度引擎中不同的参数组合可能带来不同的调度器类型，为了方便可能会创建非常多的API来满足不同场景的需要（如基础采集引擎，代理采集引擎等），但是这样API会越来越多，为了解决这种多参数配置问题，采用了函数式选项模式

不同网站有不同的处理规则，现有处理方式导致多个规则之间是割裂的；初始爬虫url与处理规则是割裂的；爬虫任务需要手动初始化才能运行；当前任务和规则都是静态的，不支持动态增加任务和任务的规则，程序无法动态的解析规则，因此需要专门管理规则的模块来完成静态规则与动态规则的解析
网站规则多种多样，无法穷尽，静态规则方式无法满足需求，不可能每次遇到新网站都要重新写代码，定义规则，重启程序，因此希望能够动态的在程序运行过程中加载规则，JavaScript虚拟机在操作网页上有天然的优势，短时间实现一个工业级的虚拟机比较困难，可以使用开源项目otto（用go编写的JavaScript虚拟机，可以在go语言中执行JavaScript语法）

不加限制的并发爬取目标网站很容易被服务器封禁，为了正常稳定的访问服务器，需要给项目增加一个限速器功能，此外如果只使用多层限速器，则访问服务器的频率会过于稳定，为了模拟人类的行为，可以在限速器的基础上增加随机休眠

GRPC的调试比HTTP烦琐，一些外部服务可能不支持使用GRPC，为了解决这些问题，需要让服务同时具备GRPC和HTTP的能力，需要借助第三方库grpc-gateway，生成一个HTTP的代理服务，将HTTP请求转换为GRPC协议，并转发到GRPC服务器中，这样服务便能同时提供HTTP接口和GRPC接口

# 功能具体实现

项目日志模块：使用Zap日志库实现项目的日志系统，Zap日志库有两种记录日志的方式，分别为高性能模式和强类型模式，前者使用`zap.Sugar()`提供类似`fmt.Printf`的友好接口，后者使用`zap.Logger`提供强类型的日志记录接口性能更高；项目中使用的是`zap.Logger`日志器来记录日志，支持将日志输出到标准输出、标准错误及文件，并提供了日志的轮转和压缩功能，通过使用lumberjack库，当日志文件达到200MB时，自动创建一个新的日志文件，并使用系统时间命名旧文件，并自动将旧文件压缩为gz格式

广度优先搜索算法爬取网站：基于广度优先算法的思想，每一层都使用当前的爬取解析函数解析网址，只会进行两层，第一层是从初始队列爬取解析话题url，第二层是从话题url中爬取正文中包含阳台字样的话题url，通过解析函数的改变可以实现在循环中的功能转换，第二层之后就不会再有第三层url添加

任务调度引擎：利用三个无缓冲通道（接收新任务的通道、分发任务给工作协程的通道、接收任务执行结果的通道）实现爬虫任务的调度，调度函数会等待接收新任务或将任务分发给工作协程，工作协程处理函数会阻塞等待任务派发，获取到派发的任务后会发起请求并解析请求，将结果通过交给处理结果的通道，对于结果的处理，首先会将结果中需要继续爬取的任务通过通道添加到工作队列中，并存储爬取到的数据；函数式选项模式，借助闭包能保持上下文环境的特性，创建一个调度引擎时，只需要选择要配置的闭包函数，而闭包函数会保持传进来的具体参数信息，并在调度引擎初始化配置信息时，用这些参数实例化一个调度引擎，实现高度可配置化可扩展性

规则引擎：

存储引擎：

限速器：基于令牌桶算法（类似饭店吃饭，有十桌，这就是桶的数量，当十桌都坐满时，新来的客户需要等待令牌，而一桌吃完之后新客户无法立即开吃，需要等待5分钟的收拾时间，这就是速率），项目中创建了多限速器（一个限速器允许2秒发送1个请求，一个限速器允许60秒发送20个请求），两个限速器按速率从小到大排序，工作协程中需要发送请求时，会调用限速器的Wait方法，该方法会遍历多限速器的所有限速器，只有当所有限速器都满足的时候才能取得令牌，否则当前协程将会阻塞直到满足条件获取到令牌，此外协程还会进行随机休眠，模拟人类的行为，防止被服务器封禁

微服务：为Worker服务构建了GRPC服务器和HTTP服务器，其中HTTP服务器使用grpc-gateway生成一个HTTP代理，最终也会访问GRPC服务器，使用go-micro微服务框架实现了GRPC服务器，同时使用go-micro插件将服务注册到etcd注册中心，
使用go-micro框架来实现GRPC服务器，使用protoc-gen-micro插件来生成micro适用的协议文件
适用grpc-gateway生成一个HTTP代理服务，将HTTP请求转换为GRPC协议发送到GRPC服务器中
proto文件决定了rpc支持的接口，rpc服务器需要实现接口中的所有方法

把worker节点变成一个支持grpc和http访问的服务，让其最终可以被master服务和外部服务直接访问：grpc默认使用protobuffer来定义接口，要使用protobuffer需要先下载proto的编译器protoc，此外还需要安装protoc的go语言插件google.golang.org/protobuf/cmd/protoc-gen-go@latest和google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest，运行protoc命令对proto文件进行编译后能得到hello.pb.go和hello_grpc.pb.go两个协议文件，此外项目中使用go-micro框架来实现grpc服务器，同样需要proto文件生成的协议文件，但是需要下载github.com/asim/go-micro/cmd/protoc-gen-micro/v4@latest插件来生成micro适用的协议文件，同样执行protoc命令可以生成micro适用的协议文件hello.pb.micro.go；proto文件决定了rpc支持的接口，rpc服务器需要实现接口中的所有方法
为了使服务具备grpc与http的能力，借助第三方库grpc-gateway，生成一个http的代理服务，会将http请求转换为grpc协议，并转发到grpc服务器中，需要对proto文件进行修改，引入依赖google/api/annotations.proto，并加入option选项，grcp-gateway的插件github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest和github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest会识别到这个自定义选项，并生成http代理服务，需要使用protoc生成新的proto协议文件，hello.pb.gw.go是grpc-gateway插件生成的文件，HandleHTTP函数会生成http服务器，监听8080端口，并将请求转发到指定的grpc服务器
项目使用etcd作为注册中心，在go-micro中使用etcd需要导入etcd插件库github.com/go-micro/plugins/v4/registry/etcd

etcd是用go实现的分布式键值存储系统，内部分为etcd-http、etcd-raft、etcd-code、etcd-server等模块，模块间用清晰的接口进行交流
etcd是分布式配置中心系统，可以作为分布式协调的组件帮忙我们实现分布式系统
etcd使用go语言编写，底层使用了Raft协议
etcd通常以分布式集群的方式部署，内部使用http通信，因此etcd抽象出了raft-http模块来负责处理和其他etcd节点间的网络通信，由于消息类型很多，心跳探活数据量小，快照信息大，因此etcd有两种处理消息的通道，分别是Pipeline消息通道和Stream消息通道，前者用于处理快照信息，处理完会关闭连接，后者用于维护节点间的长连接
etcd-raft模块是etcd的核心，实现了raft协议，可以实现节点状态的转移、节点的选举、数据处理等功能；raft节点有三中状态，分别是领导者、候选人和跟随者，etcd在raft节点新增了一个预候选人状态，并在选举之前插入了一个PreVote阶段，即当前节点会先尝试连接集群中的其他节点，只有成功连接半数以上的节点，才开始新一轮的选举
在etcd-raft模块的基础上，etcd进一步封装了raft-node模块，充当上层模块与下层raft模块之间的桥梁，并负责调用Storage模块，将记录存储到WAL日志文件以实现持久化
WAL日志的数量和大小随着时间不断增加，可能会超过可容纳的磁盘容量，同时在节点宕机后恢复数据必须从头到尾读取WAL日志文件，非常耗时，为了解决这一问题，etcd会定期创建快照并保存到文件中，恢复节点时先加载快照数据，再读取WAL文件，加快恢复速度
raft-node模块之上是etcd-server模块，其核心任务是执行Entry对应的操作，并提供限流操作与权限控制的能力，使状态机保持最新的状态，还为外部访问提供了一系列grpc api，并使用grpc-gateway支持http api能力
etcd提供了客户端工具etcdctl和clientv3代码库，支持使用grpc协议与etcd服务器交互

Master开发，由于worker和master有许多可以公用的代码，因此将worker和master放到同一个代码仓库中，使用cobra库提供的功能构建命令行应用程序，这种程序提供了一些选项或运行参数来控制程序的不同行为
使用toml加载配置：语法简洁易懂，toml采用键值对的形式，支持分层结构，使得配置信息的组织清晰明了
通过Makefile将常用的开发任务，如编译、代码检查等，封装成目标，避免了手动输入大量命令的繁琐过程，此外Makefile可以根据文件的修改时间和依赖关系，在修改代码后只编译那些变化的文件，避免要重新编译整个项目，编译的目的是将项目源代码编译成可执行文件，并注入版本信息，方便版本管理和问题排查
master和worker节点都是微服务，都暴露了grpc和http服务，master暴露的服务主要供worker调用，用于任务分配、状态同步等操作，而worker暴露的服务可以供多个对象调用，这样master节点能够实时掌握整个集群的工作进度，合理的进行任务调度和资源分配

master高可用的实现，借助etcd实现分布式服务选主：为了故障恢复，保障服务连续性
对于分布式服务，只有一个master可以成为leader，只有leader能够分配任务、处理外部访问，当leader崩溃时，其他master将竞争上岗成为新的leader，实现master的高可用
当第1个master服务上线时，即执行`go run main.go master --id=1 --http=:8081 --grpc=:9091`时首先会调用cmd下的master.go程序，从config.toml中读取数据，构建master实例，并启动当前master实例的grpc服务和http服务，构建master实例时，还会执行master下的master.go程序，初始化etcd客户端用于服务选主，从注册中心获取所有worker节点，将种子资源（也即是初始任务）添加到当前master服务中，由其进行任务分配，最后开启协程持续进行领导者选举，开启协程持续进行消息处理
在领导者选举中，基于etcd客户端的能力进行服务选主，由于当前只有一个master，因此该master服务成为主master服务，并开始监听领导者变化信息和worker节点变化信息，在有新worker节点上线、更新或下线时，通过通道传递变更信息，及时更新master维护的worker节点列表，实现worker节点服务发现
当第2个master服务上线时，同样会构建master实例，启动grpc服务和http服务，尝试通过etcd的选举机制加入选举，底层是竞争同一个key（如/resource/election）此时由于第1个master已经是领导者，第2个master不会竞选成功，并且会每隔20秒检查一下leader状态防止脑裂现象，但也能监听领导者变化信息和监听worker节点变化信息，但只有leader才能处理资源分配和任务调度（为什么只有leader才能处理资源分配和任务调度，代码中如何体现？）
当leader节点宕机时？
当上线一个worker服务时，如何注册到注册中心？——在初始化微服务的时候会自动将服务注册到注册中心
还有其他分布式协调组件，如zookeeper、consul等，为什么选择etcd？
关于选主、服务发现有很多问题可以讨论的？
go-micro、etcd、相关插件等，他们之间的关系是什么？
etcd选主原理：Campaign用了一个事务操作在要抢占的e.keyPrefix路径下维护一个Key，事务操作会首先判断当前生成的Key（如/resources/election/xxxxx）是否在etcd中，只有当结果为否的时候才会创建该Key，这样每个master都会在/resources/election/下维护一个Key，并且当前的Key是带租约的，然后Campaign会调用waitDeletes函数阻塞等待，直到自己成为leader。waitDeletes函数会调用client.Get获取当前争抢的/resources/election/路径下具有最大版本号的Key，并调用waitDeletes函数等待该Key被删除，而waitDeletes会调用client.Watch来完成对特定版本Key的监听。即当前master需要监听这个最大版本号Key的删除事件，如果这个特定的Key被删除，就意味着没有比当前Master创建的Key更早的Key了，因此当前的master理所当然成为leader，这种方式还避免了惊群效应，当leader崩溃后，并不会唤醒所有在选举中的master，只有队列中的前一个master创建的Key被删除，当前的master才会被唤醒，即所有的master都在排队等待前一个master退出，从而master以最小的代价实现了对Key的抢占

master任务调度的实现，借助micro提供的registry实现master服务发现（即master需要感知worker节点的注册与销毁）和worker节点信息维护、master资源管理：为了自动感知节点上下限，自动剔除不可用节点
在master和worker启动时，会调用service.Init()和service.Run()，go-micro会自动将服务注册到etcd，并维护心跳，确保服务在线状态；
master的服务发现借助了registry.Watch方法监听worker服务名称的变更，当有新的worker注册或下线时，会通过通道接收变更事件，更新本地worker节点列表，进而实现master对worker的感知
master通过通道接收worker节点的变更事件，当有变更事件发生时，会维护worker节点信息，获取全量的worker节点信息，并与本地的worker节点信息进行比较，更新本地的worker节点信息，实现worker节点信息的维护
master资源管理把爬虫任务当作一种资源，在初始化时，程序通过读取配置文件将爬虫任务注入master中，初始化master时调用AddSeed函数添加资源，调用AddResource将任务存储到etcd中，添加资源时使用snowflake库使用雪花算法为资源生成了一个单调递增的分布式id，雪花算法是分布式系统中生成唯一id的通用方法，其会生成一个64位的唯一id，其中41位存储时间戳，10位存储masterid，最后12位存储序列号，一毫秒内生成多个id时序列号会递增1，因此雪花算法在一毫秒内最多可以生成4096个不同的id，保证资源id全局唯一
应该先启动worker节点，再启动master节点，否则会由于没有对应的worker节点而添加master节点失败


故障容错：在worker崩溃时重新调度

完善核心能力：master请求转发与worker资源管理

服务治理：限流、熔断器、认证与鉴权
当worker节点批量请求master时，grpc协议保障高效通信，当master响应延迟上升时，熔断机制阻止worker继续发送请求，服务端限流保证master核心业务不受影响







一些认知：
yaml文件是一种数据序列化语言，专为配置文件设计，用于定义集群资源对象，是Kubernetes能理解的部署说明书
toml文件是应用程序实际读取的配置文件格式
yaml和toml结合使用，实现职责分离，yaml管理资源部署，用于Kubernetes层，toml用于管理业务参数，用于应用层
yaml是Kubernetes中定义资源对象的主要配置文件格式，用于声明式地描述集群中的各种资源，Kubernetes通过解析yaml文件来创建和管理这些资源，yaml通过缩进层级结构阻止配置信息，具有apiVersion（指定Kubernetes API版本）、kind（定义资源类型）、metadata（包含名称、标签等元数据）、spec（详细描述资源的具体规格）等关键字段

整体流程：




服务注册、服务选主、服务发现、资源管理（master和worker）、故障容错、服务治理、服务容器化、多容器部署、Kubernetes集群

故障容错功能，即在worker崩溃时重新调度
master资源调度时机：master成为leader（leader切换不常见，要全量更新当前worker节点状态与资源）、worker节点发生变化、客户端调用master api进行资源增删改查
1、master成为leader时
首先全量加载当前worker节点，再全量加载当前的爬虫资源，遍历资源，若有资源还没有被分配到节点，则尝试将资源分配到worker节点中，若资源都已经分配给对应的worker节点，则遍历查看当前节点是否存货，若当前节点已经不存在，则将该资源分配给其他节点


组件功能：
1. go-micro.dev/v4/registry是go-micro框架的服务注册发现模块，主要实现了服务注册、服务发现、状态监听等功能，在项目中，worker节点在启动时会










**梳理思路：**
1. 关于故障容错功能
明确master进行资源调度的时机有三中情况，对于第一种情况，当master成为leader时，会调用BecomeLeader方法，在该方法中，会调用updateWorkNodes方法（）加载全量worker节点，再调用loadResource方法（）加载全量资源（资源代表什么意思？），最后调用reAssign方法（）重新分配资源并原子设置master为leader状态。那么在updateWorkNodes方法中是如何加载全量worker节点的，首先会调用registry（registry在项目中有什么作用）的GetService方法（）从注册中心查询所有worker服务节点（这里一个服务节点具体代表什么，为什么是遍历services[0]中的所有结点？）

资源代表什么？————到加载资源loadResource方法中看，通过调用etcd客户端（etcd客户端在项目中有什么作用）的Get方法从etcd中获取所有资源信息（etcd资源存储形式，存储到内存还是怎么，是持久存储还是缓存？），资源在etcd中存储形式如下几个字段，并通过RESOURCEPATH=/resources进行逻辑分区和数据路由，进而可以根据路径找到不同类型的资源，资源在etcd中以json编码形式存在，取出后要进行json反序列化，ResourceSpec结构体专门用于存储资源，而master结构体也有一个资源列表字段，用于存储不同的资源（Name有啥含义，为什么要把worker2和IP配在一起，这个IP是什么ip）（资源是在什么时候存储的）（为什么会有addResources和AddResource，这两个添加资源的方法有什么区别），要先理解清楚grpc和proto才能理解里面整个的通信逻辑，那么先集中花时间把手写rpc项目搞定，再回来看代码，可能有不同的理解
```json
{
  "ID": "1525512679494094849", 
  "Name": "test.site",
  "AssignedNode": "worker2|192.168.1.11:8080",
  "CreationTime": 1718000001123456789
}
```
addResources方法和AddResource方法的区别，前者是内部方法（面向存储），后者是外部方法（面向业务）（谁进行调用？），AddResource方法用于处理单个资源请求，首先会

registry在项目中有什么作用？————这是go-micro框架的服务注册发现模块，主要实现了服务注册即worker结点启动时注册自身元数据（不是注册到etcd中吗？）、服务发现即master结点通过GetService方法获取可用worker列表、状态监听即通过Watch方法实时感知Worker上下线（这些功能具体都是怎么实现的？）


爬虫资源指什么


2. grpc+protobuf
protobuf是一种结构化数据存储格式，性能效率优于json、xml，以二进制存储占用空间小可读性差
grpc是远程过程调用系统，可以实现微服务间通信，默认使用protobuf协议进行rpc调用






本次更新做了什么：实现故障容错、完善核心能力，进一步抽象封装优化代码逻辑

问题：
为什么在cmd/master.go中不定义Greeter结构体及其Hello方法？
资源在etcd进行持久化存储和在内存中进行缓存，如何解决双写一致性问题？
代码其实有很多可以优化的地方，是否要优化？————要发现这些问题，但不改代码调试了，只要会回答即可，能够自圆其说
服务注册、服务选主、服务发现、资源管理（master和worker）、故障容错、服务治理、服务容器化、多容器部署、Kubernetes集群



简历按使用了什么技术，实现了什么功能，解决了什么问题/提升了多少性能

待做：
需要对整个流程画图加强理解
总结项目使用到的结构体与方法，梳理项目整体架构，阅读剩下的书本内容
对每个代码片段都这样问：当前代码实现中，使用了什么技术，实现了什么功能，解决了什么问题，存在哪些技术难点，又是如何解决的​
将项目整个都发给deepseek帮忙总结形成简历


我将把我做的项目的全部代码文件发给你，这个项目是一个高性能、分布式爬虫项目，该项目包含以下目录结构：根目录下包含auth目录、cmd目录、extensions目录、generator目录、kubernetes目录、limiter目录、log目录、master目录、proto目录、proxy目录、spider目录、sqlstorage目录、tasklib目录、version目录、以及.golangci.yml文件、config.toml文件、docker-compose.yml文件、Dockerfile文件、Makefile文件、main.go文件，其中auth目录包含auth.go代码文件，cmd目录下包含master目录和worker目录以及cmd.go代码文件，cmd目录下的master目录中包含master.go代码文件，cmd目录下的worker目录中包含worker.go代码文件，extensions目录包含randomua.go代码文件，generator目录包含generator.go代码文件，kubernetes目录包含configmap.yaml、crawl-master-service.yaml、crawl-master.yaml、crawl-worker-service.yaml、crawl-worker.yaml、ingress.yaml代码文件，limiter目录包含limiter.go代码文件，log目录包含default.go、file_test.go、log.go代码文件，master目录包含master.go、option.go代码文件，proto目录包含crawler目录和greeter目录，crawler目录包含crawler.proto、crawler_grpc.pb.go、crawler.pb.go、crawler.pb.gw.go、crawler.pb.micro.go代码文件，greeter目录包含hello.proto、hello_grpc.pb.go、hello.pb.go、hello.pb.gw.go、hello.pb.micro.go代码文件，proxy目录包含proxy.go、proxy_test.go代码文件，spider目录包含workerengine目录、context.go、datarepository.go、fetchservice.go、request.go、requestrepository.go、resource.go、resourceregistry.go、resourcerepository.go、rule.go、task.go、taskstore.go、temp.go代码文件，workerengine目录包含option.go、scheduleservice.go、workerservice.go代码文件，sqlstorage目录包含sqldb目录、option.go、sqlstorage_test.go、sqlstorage.go代码文件，sqldb目录包含option.go、sqldb_test.go、sqldb.go代码文件，tasklib目录包含doubanbook目录、doubangroup目录、doubangroupjs目录、tasklib.go代码文件，doubanbook目录包含book.go代码文件，doubangroup目录包含group.go代码文件，doubangourpjs目录包含groupjs.go代码文件，version目录包含version.go代码文件

现在我准备写一份求职岗位为golang开发工程师的简历，准备将这个爬虫项目作为一个项目经历写进简历中，你会怎么写，请给出项目描述和个人职责（即使用什么技术实现了什么功能解决了什么问题）以及技术难点（使用什么技术/方法解决了什么业务难点），你要严格根据项目代码实现来给出内容，不能自己胡编乱造

repository存储库的意思










多关注技术选型：为什么用这家而不是另外的



