# 高性能、分布式爬虫项目


## 前置知识

首先是爬虫具有很高的商业价值，借助爬虫可以创造种类繁多的商业模式
其次爬虫对高并发的网络处理有极高的要求，Go语言具有天然的优势

学习目标：
能够理解每句代码的含义，理解项目的总体设计，并能够回答常见的对于项目的问题即可
看每一部分的时候都要思考：这部分要看吗，在整个项目中起到了什么作用，要了解到什么程度
写代码的时候，对于学过的部分要尝试自己写，然后对照答案，对于没学过的就先看然后再自己写
Go之所以简单，是因为它采用同步的方式来处理网络I/O；Go之所以高效，是因为在同步处理的表象下，Go运行时封装I/O多路复用，并灵活地调度协程以实现异步处理，从而充分利用CPU等资源


Go高并发的网络模型：协程调度 + 同步编程模式 + 非阻塞I/O + I/O多路复用
协程调度引入轻量级的协程，降低了线程的时间成本（切换线程上下文）和线程的空间成本（线程堆栈大小较大，限制了创建线程的数量）
同步编程模式：读写数据陷入等待直到数据读写完毕，直观上是同步的，简化了处理流程
非阻塞I/O：开发者态（协程）阻塞，线程不阻塞，从而通过调度协程，确保了高并发性能

Go高性能：系统级别、程序设计与组织级别、代码实施级别、操作系统级别、硬件级别

Go微服务
微服务是一种软件架构风格
单体服务从按功能/逻辑划分模块（分层架构），到在分层的基础上按业务功能做拆分（垂直切片），模块间通过接口进行通信，但随着业务复杂度的上升，容易产生很多问题
将服务拆分为职责单一、功能细粒度的微服务解决了以上的问题
微服务的边界：高内聚、低耦合
微服务的通信：同步通信（阻塞等待服务器的返回）、异步回调、异步事件驱动（向消息中间件发送消息，消息由下游服务消费）、异步数据共享（通过共享的数据库或文件进行通信，下游轮询变更）
微服务动态变更与销毁————服务发现：服务端发现（由代理网关选定合适的被调用方）、客户端发现（由调用方监听服务结点注册变化进而动态选择合适的被调用方）
微服务问题与解决：
1. 收集分散在各处的服务日志（信息聚合）
2. 聚合服务的各种信息（信息聚合）
3. 及时感知服务与服务之间的调用关系，追踪调用链
4. 微服务测试：单元测试、服务测试和端到端测试
5. 服务降级：降级、限流、熔断、切流
微服务是分布式架构的一种特例，同样面临着分布式系统的数据一致性（网络延迟、网络分区、系统故障、不可靠的时钟）和可用性问题
CAP定理：线性一致性（更新完成后，任何后续的访问都会返回更新的值）、可用性（有失败结点，其他结点仍能正常工作并响应）和分区容忍度（能够荣仍任意数量的消息丢失）这三个属性不能同时获得

## 爬虫项目分析与设计

### 爬虫的合法性

爬虫之前需要提前确定爬取数据的访问权限，如（1）数据发布者已决定将数据公开，如暴露了API；（2）开发者无须创建账户或登录即可访问的数据；（3）该网站的robots.txt文件允许访问的数据

### 爬虫的商业价值

信息聚合，更快获取准确有用的信息，用于投资方面

行业见解

预测

机器学习，爬取训练数据


### 需求分析

业务需求：构建一个以爬虫引擎为基础的推送系统，为资本市场上的开发者快速提供热点事件和事件预警

开发者需求：


# 学习笔记

Go语言正则表达式默认使用PCRE标准，也支持POSIX标准
XPath定义了一种遍历XML文档中结点层次结构，并返回匹配元素的灵活方法
XML是一种可扩展是标记语言，标识结构化信息的一种规范
CSS是一种定义HTML文档中元素样式的语言，在CSS文件中可以定义一个或多个HTML中的标签的路径，并指定这些标签的样式，定义标签路径的方法被成为CSS选择器
Go标准库使用连接池来优化获取连接的过程，①对于请求结束后的连接，会根据是否有等待连接的协程，若有则唤醒等待的协程并使用该连接发起请求，若没有则会将该连接放入连接池中；②对于请求开始时，会先从连接池中查看是否有对应的空闲连接，若有则使用该连接发起请求，若没有则该协程会阻塞等待，会通过异步方式与服务器建立连接，在这个过程中若有连接正好空闲，则该协程会优先使用该空闲连接，提高使用效率


# 问题

1. 直接Get请求得到的响应内容中只有页面一部分的响应内容，其余部分正在加载，因此响应内容不是全部的内容，没有包含澎湃热榜的内容


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


# 功能具体实现

项目日志模块：使用Zap日志库实现项目的日志系统，Zap日志库有两种记录日志的方式，分别为高性能模式和强类型模式，前者使用`zap.Sugar()`提供类似`fmt.Printf`的友好接口，后者使用`zap.Logger`提供强类型的日志记录接口性能更高；项目中使用的是`zap.Logger`日志器来记录日志，支持将日志输出到标准输出、标准错误及文件，并提供了日志的轮转和压缩功能，通过使用lumberjack库，当日志文件达到200MB时，自动创建一个新的日志文件，并使用系统时间命名旧文件，并自动将旧文件压缩为gz格式

广度优先搜索算法爬取网站：基于广度优先算法的思想，每一层都使用当前的爬取解析函数解析网址，只会进行两层，第一层是从初始队列爬取解析话题url，第二层是从话题url中爬取正文中包含阳台字样的话题url，通过解析函数的改变可以实现在循环中的功能转换，第二层之后就不会再有第三层url添加

任务调度引擎：利用三个无缓冲通道（接收新任务的通道、分发任务给工作协程的通道、接收任务执行结果的通道）实现爬虫任务的调度，调度函数会等待接收新任务或将任务分发给工作协程，工作协程处理函数会阻塞等待任务派发，获取到派发的任务后会发起请求并解析请求，将结果通过交给处理结果的通道，对于结果的处理，首先会将结果中需要继续爬取的任务通过通道添加到工作队列中，并存储爬取到的数据；函数式选项模式，借助闭包能保持上下文环境的特性，创建一个调度引擎时，只需要选择要配置的闭包函数，而闭包函数会保持传进来的具体参数信息，并在调度引擎初始化配置信息时，用这些参数实例化一个调度引擎，实现高度可配置化可扩展性


# 学习进度

1.4 v0.0.4抄了，但是还没理解代码怎么写的






