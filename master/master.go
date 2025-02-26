package master

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/dszqbsm/crawler/cmd/worker"
	"go-micro.dev/v4/registry"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.uber.org/zap"
)

//

const (
	RESOURCEPATH = "/resources"
)

// 用于表示一个master节点
type Master struct {
	ID        string                    // master节点id
	ready     int32                     // 是否为领导者状态
	leaderID  string                    // 领导者id
	workNodes map[string]*registry.Node // 工作节点列表
	resources map[string]*ResourceSpec  // 资源列表
	IDGen     *snowflake.Node           // ID生成器
	etcdCli   *clientv3.Client          // etcd客户端
	options
}

// 用于判断当前master节点是否为领导者
func (m *Master) IsLeader() bool {
	return atomic.LoadInt32(&m.ready) != 0
}

// 实现master节点的领导者选举和监控逻辑，在创建etcd会话和选举对象后，启动协程进行选举操作，并监听领导者变化和工作节点变化
// 在循环中处理选举结果、领导者变化和工作节点变化的消息，每20秒检查一次当前的领导者信息
func (m *Master) Campaign() {
	// etcd会话自动每5秒进行续约
	s, err := concurrency.NewSession(m.etcdCli, concurrency.WithTTL(5)) // 创建一个新的etcd会话
	if err != nil {
		fmt.Println("NewSession", "error", "err", err)
	}
	defer s.Close() // 函数返回前关闭etcd会话

	// 创建一个新的etcd选举对象election，选举键为"/resources/election"，所有的master都在抢占，以目录形式的结构作为key
	e := concurrency.NewElection(s, "/resources/election")
	leaderCh := make(chan error)                    // 创建一个用于接收选举结果的通道
	go m.elect(e, leaderCh)                         // 启动一个新协程进行选举操作，并将选举结果通过选举通道返回
	leaderChange := e.Observe(context.Background()) // 监听leader节点的变化，返回一个用于接收领导者变化信息的通道，用于监听指定选举键下键值对的变化
	// select {// 从leaderChange通道中接收领导者变化信息，并记录日志
	// case resp := <-leaderChange:
	// 	m.logger.Info("watch leader change", zap.String("leader:", string(resp.Kvs[0].Value)))
	// }
	workerNodeChange := m.WatchWorker() // 调用WatchWorker方法监听工作节点变化，返回一个用于接收工作节点变化信息的通道

	for { // 使用无限循环和select语句来处理不同通道的消息
		select {
		case err := <-leaderCh: // 从leaderCh通道中接收选举结果
			if err != nil { // 选举失败则记录错误信息并重新发起选举
				m.logger.Error("leader elect failed", zap.Error(err))
				// 这里重新发起选举
				go m.elect(e, leaderCh)
			} else { // 选举成功则记录日志，更新领导者id，并调用BecomeLeader方法使当前节点成为领导者
				m.logger.Info("master start change to leader")
				m.leaderID = m.ID
				if !m.IsLeader() {
					if err := m.BecomeLeader(); err != nil {
						m.logger.Error("BecomeLeader failed", zap.Error(err))
					}
				}
			}
		case resp := <-leaderChange: // 从leaderChange通道中接收领导者变化信息
			if len(resp.Kvs) > 0 { // 若有变化则记录日志
				m.logger.Info("watch leader change", zap.String("leader:", string(resp.Kvs[0].Value)))
			}
		case resp := <-workerNodeChange: // 从workerNodeChange通道中接收工作节点变化信息，记录日志并调用updateWorkNodes方法更新工作节点列表
			m.logger.Info("watch worker change", zap.Any("worker:", resp))
			m.updateWorkNodes()
		// 每20秒检查一次当前的领导者信息，与leader每5秒自动进行续约不同，该机制可以防止脑裂或状态不一致问题，脑裂即分布式系统集群中同时存在多个自认为合法的leader
		// 当当前leader因为网络问题无法与etcd保持通信进行续约时，就会失去leader身份，触发重新选举出新leader，但其本地仍认为自己是leader，导致双leader脑裂现象
		// 此时20秒检查能及时发现本地leader与etcd中的不一致，从而自动降级，是额外的安全措施，确保状态一致性
		case <-time.After(20 * time.Second): // 每20秒检查依次当前的领导者信息
			rsp, err := e.Leader(context.Background()) // 获取当前的领导者信息
			if err != nil {                            // 若获取领导者信息失败且没有领导者，则重新发起选举
				m.logger.Info("get Leader failed", zap.Error(err))
				if errors.Is(err, concurrency.ErrElectionNoLeader) {
					go m.elect(e, leaderCh)
				}
			}
			if rsp != nil && len(rsp.Kvs) > 0 { // 若当前节点是领导者但实际领导者已经改变，则将当前节点的领导者状态标记为非领导者
				m.logger.Debug("get Leader", zap.String("value", string(rsp.Kvs[0].Value)))
				if m.IsLeader() && m.ID != string(rsp.Kvs[0].Value) {
					//当前已不再是leader
					atomic.StoreInt32(&m.ready, 0)
				}
			}
		}
	}
}

// 执行领导者选举操作，并将选举结果通过通道返回
func (m *Master) elect(e *concurrency.Election, ch chan error) {
	// 堵塞直到选取成功，唤醒e.Campaign
	// 基于etcd的原子操作和租约机制实现领导者选举，多个节点会同时调用e.Campaign方法，竞争在选举键下创建键值对的权利，第一个成功创建的节点将成为领导者
	// 成为领导者的节点会定期续约租约，以保持其领导者地位，若领导者节点崩溃或失去与其他etcd节点的连接，租约到期后，etcd会自动删除该键值对，其他节点可以再次竞争领导者
	// Campaign方法会阻塞直到当前节点成为leader，若当前已经存在leader则会一只阻塞直到当前leader会话过期或释放leader
	err := e.Campaign(context.Background(), m.ID)
	ch <- err
}

// 监听工作节点的变更事件，并将变更结果通过通道返回
func (m *Master) WatchWorker() chan *registry.Result {
	// 创建服务监听器，通过registry监听指定worker服务名称的变更
	watch, err := m.registry.Watch(registry.WatchService(worker.ServiceName))
	if err != nil {
		panic(err)
	}
	// 创建结果通道，用于向主循环传递服务变更事件
	ch := make(chan *registry.Result)
	// 启动协程持续监听变更
	go func() {
		for {
			// 阻塞调用在协程中执行，不会阻塞主线程
			// 阻塞等待下一个变更事件，如节点注册、节点更新、节点下线等事件类型
			res, err := watch.Next()
			if err != nil {
				m.logger.Info("watch worker service failed", zap.Error(err))
				continue
			}
			ch <- res
		}
	}()
	return ch
}

// 当节点成为领导者时，加载资源并标记为领导者状态
func (m *Master) BecomeLeader() error {
	// 从etcd加载所有资源到内存map中
	// etcd中以键值对形式存储了资源信息
	if err := m.loadResource(); err != nil {
		return fmt.Errorf("loadResource failed:%w", err)
	}
	// 原子操作更新状态，保证并发安全
	atomic.StoreInt32(&m.ready, 1)
	return nil
}

// 更新master节点记录的工作节点信息，首先从服务注册中心获取所有节点的服务信息，对比新旧节点信息，找出增删改的节点，更新master节点的工作节点信息
func (m *Master) updateWorkNodes() {
	// 从注册中心查询所有worker服务节点
	services, err := m.registry.GetService(worker.ServiceName)
	if err != nil {
		m.logger.Error("get service", zap.Error(err))
	}
	// 构建节点映射标，以节点id为key
	nodes := make(map[string]*registry.Node)
	if len(services) > 0 {
		// 遍历第一个服务的所有节点
		for _, spec := range services[0].Nodes {
			nodes[spec.Id] = spec // 存储节点元数据
		}
	}

	// 计算节点差异：对比新旧节点状态，找出新增、删除和修改的节点
	added, deleted, changed := workNodeDiff(m.workNodes, nodes)
	m.logger.Sugar().Info("worker joined: ", added, ", leaved: ", deleted, ", changed: ", changed)
	// 更新master维护的节点状态
	m.workNodes = nodes
}

// 向系统中添加资源，为每个资源生成唯一id，并将资源分配给一个工作节点，记录分配信息和创建时间，再将资源信息存储道etcd中，并更新master节点的资源记录
func (m *Master) AddResource(rs []*ResourceSpec) {
	for _, r := range rs {
		// 生成唯一id
		r.ID = m.IDGen.Generate().String()
		// 分配工作节点，当前实现为直接使用第一个工作节点
		ns, err := m.Assign(r)
		if err != nil {
			m.logger.Error("assign failed", zap.Error(err))
			continue
		}
		// 记录节点分配信息
		r.AssignedNode = ns.Id + "|" + ns.Address // 格式：workerID|address
		// 记录时间戳
		r.CreationTime = time.Now().UnixNano()
		m.logger.Debug("add resource", zap.Any("specs", r))

		// 通过json序列化持久化存储到etcd中
		// 双重存储，内存存储即保存在master的resources map中，持久化存储即通过json序列化存入etcd中
		_, err = m.etcdCli.Put(context.Background(), getResourcePath(r.Name), encode(r))
		if err != nil {
			m.logger.Error("put etcd failed", zap.Error(err))
			continue
		}
		// 更新内存缓存
		m.resources[r.Name] = r
	}
}

// 处理接收到的消息，根据消息类型执行相应的操作
func (m *Master) HandleMsg() {
	msgCh := make(chan *Message)

	select {
	case msg := <-msgCh: // 阻塞等待消息
		switch msg.Cmd {
		case MSGADD: // 处理资源添加命令
			m.AddResource(msg.Specs)
		}
	}
}

// 选取一个工作节点并返回
func (m *Master) Assign(r *ResourceSpec) (*registry.Node, error) {
	// 遍历工作节点，直接返回第一个工作节点
	for _, n := range m.workNodes {
		return n, nil
	}
	return nil, errors.New("no worker nodes")
}

// 添加种子资源
func (m *Master) AddSeed() {
	rs := make([]*ResourceSpec, 0, len(m.Seeds))
	for _, seed := range m.Seeds {
		// 检查资源是否存在
		resp, err := m.etcdCli.Get(context.Background(), getResourcePath(seed.Name), clientv3.WithSerializable())
		if err != nil {
			m.logger.Error("etcd get faiiled", zap.Error(err))
			continue
		}
		if len(resp.Kvs) == 0 { // etcd中不存在该资源
			r := &ResourceSpec{
				Name: seed.Name, // 仅初始化名称，渐进式初始化
			}
			rs = append(rs, r) // 加入待添加队列
		}
	}
	// 批量添加资源
	m.AddResource(rs)
}

// 从etcd中加载所有资源信息
func (m *Master) loadResource() error {
	// 从etcd中获取所有资源信息
	resp, err := m.etcdCli.Get(context.Background(), RESOURCEPATH, clientv3.WithSerializable())
	if err != nil {
		return fmt.Errorf("etcd get failed")
	}

	resources := make(map[string]*ResourceSpec)
	// 遍历所有键值对，反序列化并存储
	for _, kv := range resp.Kvs {
		r, err := decode(kv.Value)
		if err == nil && r != nil {
			resources[r.Name] = r
		}
	}
	m.logger.Info("leader init load resource", zap.Int("lenth", len(m.resources)))
	// 更新资源列表
	m.resources = resources
	return nil
}

// 用于表示消息的命令类型
type Command int

const (
	// 添加资源命令
	MSGADD Command = iota // iota从0开始
	// 删除资源命令
	MSGDELETE // 自动递增为1
)

// 封装消息，包含命令类型和资源规格
type Message struct {
	Cmd   Command
	Specs []*ResourceSpec
}

// 资源类型定义
type ResourceSpec struct {
	ID           string // 资源id
	Name         string // 资源名称
	AssignedNode string // 格式为"Worker节点ID|节点地址"
	CreationTime int64  // 创建时间
}

// 将结构体对象编码为json字符串
func encode(s *ResourceSpec) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// 根据资源名称生成资源在etcd中的存储路径
func getResourcePath(name string) string {
	return fmt.Sprintf("%s/%s", RESOURCEPATH, name)
}

// 将json字符串解码为结构体对象
func decode(ds []byte) (*ResourceSpec, error) {
	var s *ResourceSpec
	err := json.Unmarshal(ds, &s)
	return s, err
}

// 对比新旧工作节点信息，找出增删改的节点id
// 两次遍历总体时间复杂度：O(n+m)优于双重循环
func workNodeDiff(old map[string]*registry.Node, new map[string]*registry.Node) ([]string, []string, []string) {
	added := make([]string, 0)
	deleted := make([]string, 0)
	changed := make([]string, 0)
	// 第一轮遍历：检测新增和变更的节点
	for k, v := range new {
		if ov, ok := old[k]; ok { // 若节点在旧集合中存在
			if !reflect.DeepEqual(v, ov) { // 深层比较节点对象
				changed = append(changed, k)
			}
		} else { // 若节点不在旧集合中，则为新增节点
			added = append(added, k)
		}
	}
	// 第二轮遍历：检测删除的节点
	for k := range old {
		if _, ok := new[k]; !ok { // 旧节点在新集合中不存在
			deleted = append(deleted, k)
		}
	}
	return added, deleted, changed
}

// 获取本机网卡IP
func getLocalIP() (string, error) {
	var (
		addrs []net.Addr
		err   error
	)
	// 获取所有网卡
	if addrs, err = net.InterfaceAddrs(); err != nil {
		return "", err
	}
	// 取第一个非lo的网卡IP
	// 可能会取到非预期的ip，比如docker虚拟网卡
	for _, addr := range addrs {
		if ipNet, isIpNet := addr.(*net.IPNet); isIpNet && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String(), nil
			}
		}
	}

	return "", errors.New("no local ip")
}

// 创建一个新的master实例
// 使用snowflake雪花算法生成唯一id、
func New(id string, opts ...Option) (*Master, error) {
	m := &Master{} // 创建master空实例

	// 合并配置选项
	options := defaultOptions
	for _, opt := range opts {
		opt(&options)
	}
	// 注入配置
	m.options = options
	m.resources = make(map[string]*ResourceSpec)

	// 初始化雪花算法id生成器
	node, err := snowflake.NewNode(1)
	if err != nil {
		return nil, err
	}
	// 注入id生成器
	m.IDGen = node
	// 获取本机ipv4地址
	ipv4, err := getLocalIP()
	if err != nil {
		return nil, err
	}
	// 组合多个标识参数生成唯一masterid
	m.ID = genMasterID(id, ipv4, m.GRPCAddress)
	m.logger.Sugar().Debugln("master_id:", m.ID)

	// 初始化etcd客户端
	endpoints := []string{m.registryURL}
	cli, err := clientv3.New(clientv3.Config{Endpoints: endpoints})
	if err != nil {
		return nil, err
	}
	// 注入etcd客户端
	m.etcdCli = cli

	// 初始化工作节点列表
	m.updateWorkNodes() // 从注册中心获取worker节点
	m.AddSeed()         // 添加种子资源
	go m.Campaign()     // 启动协程进行领导者选举
	go m.HandleMsg()    // 启动协程处理消息
	return m, nil
}

// 用于生成masterid
func genMasterID(id string, ipv4 string, GRPCAddress string) string {
	return "master" + id + "-" + ipv4 + GRPCAddress
}
