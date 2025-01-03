
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