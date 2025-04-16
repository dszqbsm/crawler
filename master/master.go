package master

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/snowflake"
	proto "github.com/dszqbsm/crawler/proto/crawler"
	"github.com/golang/protobuf/ptypes/empty"
	"go-micro.dev/v4/client"
	"go-micro.dev/v4/registry"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.uber.org/zap"
)

//

const (
	RESOURCEPATH = "/resources"

	ADDRESOURCE = iota
	DELETERESOURCE

	ServiceName = "go.micro.server.worker"
)

// 用于表示一个master节点
type Master struct {
	ID       string // master节点id
	ready    int32  // 是否为领导者状态
	leaderID string // 领导者id
	/* workNodes map[string]*registry.Node // 工作节点列表 */
	workNodes  map[string]*NodeSpec
	resources  map[string]*ResourceSpec // 资源列表
	IDGen      *snowflake.Node          // ID生成器
	etcdCli    *clientv3.Client         // etcd客户端
	forwardCli proto.CrawlerMasterService
	rlock      sync.Mutex
	options
}

/*
输入一个grpc客户端代理，无输出

该方法用于向master结构体注入一个grpc客户端代理，使得master能够通过此客户端与其他服务（如其他master节点或worker节点）进行通信

允许在运行时动态设置或替换master的通信客户端，解耦了master与具体客户端实现的依赖关系
*/
func (m *Master) SetForwardCli(forwardCli proto.CrawlerMasterService) {
	m.forwardCli = forwardCli // 注入grpc客户端代理
}

/*
输入一个上下文、一个资源规格和一个空结构体，输出一个错误

该方法用于在分布式系统中删除资源，若当前节点不是leader节点且leader节点存在且非本身，则将删除请求通过grpc客户端转发给leader节点，确保只有leader节点处理删除操作，维护数据一致性

若当前节点是leader节点，先加锁，检查资源是否存在，存在则从etcd中删除持久化数据，再清理内存中的资源，最后更新工作节点负载

不懂：若为非leader节点，是如何将请求转发给leader节点的？
*/
func (m *Master) DeleteResource(ctx context.Context, spec *proto.ResourceSpec, empty *empty.Empty) error {
	// 非leader节点转发请求给leader
	if !m.IsLeader() && m.leaderID != "" && m.leaderID != m.ID {
		addr := getLeaderAddress(m.leaderID)
		// 通过grpc客户端转发删除请求
		_, err := m.forwardCli.DeleteResource(ctx, spec, client.WithAddress(addr))
		return err
	}

	// leader节点处理逻辑

	// 加写锁保证原子性
	m.rlock.Lock()
	defer m.rlock.Unlock()

	// 检查资源是否存在
	r, ok := m.resources[spec.Name]

	if !ok {
		return errors.New("no such task")
	}

	// 从etcd删除持久化数据
	if _, err := m.etcdCli.Delete(ctx, getResourcePath(spec.Name)); err != nil {
		return err
	}

	// 从内存map删除资源
	delete(m.resources, spec.Name)

	// 更新工作节点负载
	if r.AssignedNode != "" {
		nodeID, err := getNodeID(r.AssignedNode) // 解析节点id
		if err != nil {
			return err
		}
		// 减少对应节点的负载计数
		if ns, ok := m.workNodes[nodeID]; ok {
			atomic.AddInt32(&ns.Payload, -1) // 原子减少负载计数
		}
	}

	return nil
}

/*
输入一个资源规格，输出分配的节点信息和错误

该方法用于在分布式系统中添加资源，首先通过雪花算法为该资源生成分布式唯一id，接着将该资源基于负载均衡算法分配给一个工作节点，在检查节点有效之后记录节点分配信息，并持久化到etcd中，最后将数据同步到内存中，并更新节点负载
*/
func (m *Master) addResources(r *ResourceSpec) (*NodeSpec, error) {
	// 雪花算法生成唯一资源id
	r.ID = m.IDGen.Generate().String()
	// 分配工作节点
	ns, err := m.Assign(r)
	if err != nil {
		m.logger.Error("assign failed", zap.Error(err))
		return nil, err
	}

	// 检查节点有效性
	if ns.Node == nil {
		m.logger.Error("no node to assgin")
		return nil, err
	}

	// 记录节点分配信息
	r.AssignedNode = ns.Node.Id + "|" + ns.Node.Address
	r.CreationTime = time.Now().UnixNano()
	m.logger.Debug("add resource", zap.Any("specs", r))

	// 持久化到etcd
	_, err = m.etcdCli.Put(context.Background(), getResourcePath(r.Name), encode(r))
	if err != nil {
		m.logger.Error("put etcd failed", zap.Error(err))
		return nil, err
	}
	// 更新内存状态
	m.rlock.Lock() // 加锁
	defer m.rlock.Unlock()
	m.resources[r.Name] = r
	atomic.AddInt32(&ns.Payload, 1) // 原子增加节点负载计数
	return ns, nil
}

/*
无输入，输出一个布尔值

方方法用于检查当前节点是否为leader节点，原子检查m.ready的值，为0表示非leader节点，非0表示leader节点
*/
func (m *Master) IsLeader() bool {
	return atomic.LoadInt32(&m.ready) != 0
}

/*
无输入，无输出

该方法用于leader选举、状态监控与同步、故障恢复

创建一个etcd会话后，创建一个etcd选举对象，启动协程进行选举，并通过通道接收选举结果，通过通道监听leader节点和worker节点变更

for无限循环处理不同通道的信息，对于选举结果通道，若选举失败则重新发起选举，选举成功则更新leaderID和调用BecomeLeader方法使当前节点成为leader

对于leader节点变更通道，若有变化则更新leaderID，并检查当前节点是否已经不再是leader，若是则将m.ready置为0

对于worker节点变更通道，若有变化则调用updateWorkNodes方法更新工作节点列表，并调用loadResource方法加载资源，最后调用reAssign方法重新分配资源

最后每20秒检查一次当前leader的状态，防止脑裂问题，若获取leader节点信息失败且当前没有leader节点，则重新发起选举，若当前节点是leader但实际leader已变更，则将m.ready置为0
*/
func (m *Master) Campaign() {
	// etcd会话自动每5秒进行续约
	s, err := concurrency.NewSession(m.etcdCli, concurrency.WithTTL(5)) // 创建一个新的etcd会话
	if err != nil {
		fmt.Println("NewSession", "error", "err", err)
	}
	defer s.Close() // 函数返回前关闭etcd会话

	// 创建一个新的etcd选举对象election，选举键为"/resources/election"，所有的master都在抢占，以目录形式的结构作为key
	e := concurrency.NewElection(s, "/crawler/election")
	leaderCh := make(chan error)                    // 创建一个用于接收选举结果的通道
	go m.elect(e, leaderCh)                         // 启动一个新协程进行选举操作，并将选举结果通过选举通道返回
	leaderChange := e.Observe(context.Background()) // 监听leader节点的变化，返回一个用于接收领导者变化信息的通道，用于监听指定选举键下键值对的变化
	workerNodeChange := m.WatchWorker()             // 调用WatchWorker方法监听工作节点变化，返回一个用于接收工作节点变化信息的通道

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
				m.leaderID = string(resp.Kvs[0].Value)
				if m.ID != string(resp.Kvs[0].Value) {
					//当前已不再是leader
					atomic.StoreInt32(&m.ready, 0)
				}
			}
		case resp := <-workerNodeChange: // 从workerNodeChange通道中接收工作节点变化信息，记录日志并调用updateWorkNodes方法更新工作节点列表
			m.logger.Info("watch worker change", zap.Any("worker:", resp))
			m.updateWorkNodes()
			if err := m.loadResource(); err != nil {
				m.logger.Error("loadResource failed:%w", zap.Error(err))
			}
			if m.IsLeader() {
				m.reAssign() // 仅在当前节点是Leader时执行资源重分配
			}
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

/*
输入一个etcd选举对象和一个错误通道，无输出

该方法用于进行选举操作，会阻塞直到当前节点成为leader
*/
func (m *Master) elect(e *concurrency.Election, ch chan error) {
	// 堵塞直到选取成功，唤醒e.Campaign
	// 基于etcd的原子操作和租约机制实现领导者选举，多个节点会同时调用e.Campaign方法，竞争在选举键下创建键值对的权利，第一个成功创建的节点将成为领导者
	// 成为领导者的节点会定期续约租约，以保持其领导者地位，若领导者节点崩溃或失去与其他etcd节点的连接，租约到期后，etcd会自动删除该键值对，其他节点可以再次竞争领导者
	// Campaign方法会阻塞直到当前节点成为leader，若当前已经存在leader则会一只阻塞直到当前leader会话过期或释放leader
	err := e.Campaign(context.Background(), m.ID)
	ch <- err
}

/*
无输入，输出一个结果通道

该方法用于监听worker节点的变更，启动协程持续阻塞监听worker节点的变更
*/
func (m *Master) WatchWorker() chan *registry.Result {
	// 创建服务监听器，通过registry监听指定worker服务名称的变更
	watch, err := m.registry.Watch(registry.WatchService(ServiceName))
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

/*
无输入，输出一个错误

该方法用于将当前节点设置为leader节点，首先更新工作节点信息，然后全量加载资源，最后重新分配资源
*/
func (m *Master) BecomeLeader() error {
	m.updateWorkNodes()
	if err := m.loadResource(); err != nil {
		return fmt.Errorf("loadResource failed:%w", err)
	}

	m.reAssign()
	// 原子操作更新状态，保证并发安全
	atomic.StoreInt32(&m.ready, 1)
	return nil
}

/*
无输入，无输出

该方法用于更新worker节点列表，首先从注册中心查询所有worker节点存储到内存中，接着对比新旧节点状态，找出新增、删除和修改的节点，更新master维护的节点状态
*/
func (m *Master) updateWorkNodes() {
	// 从注册中心查询所有worker服务节点
	services, err := m.registry.GetService(ServiceName /* worker.ServiceName */)
	if err != nil {
		m.logger.Error("get service", zap.Error(err))
	}
	m.rlock.Lock()
	defer m.rlock.Unlock()
	// 构建节点映射标，以节点id为key
	nodes := make(map[string]*NodeSpec)
	if len(services) > 0 {
		// 遍历第一个服务的所有节点
		for _, spec := range services[0].Nodes {
			/* nodes[spec.Id] = spec */ // 存储节点元数据
			nodes[spec.Id] = &NodeSpec{
				Node: spec,
			}
		}
	}

	// 计算节点差异：对比新旧节点状态，找出新增、删除和修改的节点
	added, deleted, changed := workNodeDiff(m.workNodes, nodes)
	m.logger.Sugar().Info("worker joined: ", added, ", leaved: ", deleted, ", changed: ", changed)
	// 更新master维护的节点状态
	m.workNodes = nodes
}

/*
输入一个上下文、一个资源规格和一个分配的节点信息，输出一个错误

该方法用于在分布式系统中新增资源，若当前节点不是leader节点且leader节点存在且非本身，则将新增请求通过grpc客户端转发给leader节点，确保只有leader节点处理新增操作，维护数据一致性

若当前节点是leader节点，先加锁，接着使用雪花算法为该资源生成唯一id，接着将该资源基于负载均衡算法分配给一个工作节点，在检查节点有效之后记录节点分配信息，并持久化到etcd中，最后将数据同步到内存中，并更新节点负载，最后更新分配的节点信息

不懂：若为非leader节点，是如何将请求转发给leader节点的？
*/
func (m *Master) AddResource(ctx context.Context, req *proto.ResourceSpec, resp *proto.NodeSpec) error {
	// 非leader节点，请求转发
	if !m.IsLeader() && m.leaderID != "" && m.leaderID != m.ID {
		addr := getLeaderAddress(m.leaderID)
		nodeSpec, err := m.forwardCli.AddResource(ctx, req, client.WithAddress(addr))
		resp.Id = nodeSpec.Id
		resp.Address = nodeSpec.Address
		return err
	}

	// leader节点处理逻辑
	m.rlock.Lock()
	defer m.rlock.Unlock()
	nodeSpec, err := m.addResources(&ResourceSpec{Name: req.Name})
	if nodeSpec != nil {
		resp.Id = nodeSpec.Node.Id
		resp.Address = nodeSpec.Node.Address
	}
	return err
}

/*
输入一个leader节点地址，输出leader节点的IP地址

该方法用于解析leader节点地址，提取出IP地址
*/
func getLeaderAddress(address string) string {
	s := strings.Split(address, "-")
	if len(s) < 2 {
		return ""
	}
	return s[1]
}

/*
输入一个资源规格，输出分配的节点信息和错误

该方法用于在分布式系统中基于负载均衡算法为资源分配一个工作节点，首先创建一个候选节点切片，容量为当前工作节点总数，然后遍历所有工作节点，收集候选节点，接着按节点负载升序排序，选择负载最低的节点，若存在候选节点，则返回负载最低的节点，否则返回错误
*/
func (m *Master) Assign(r *ResourceSpec) (*NodeSpec, error) {
	// 创建候选节点切片，容量为当前工作节点总数
	candidates := make([]*NodeSpec, 0, len(m.workNodes))

	// 遍历所有工作节点，收集候选节点
	for _, node := range m.workNodes {
		candidates = append(candidates, node)
	}

	// 按节点负载升序排序
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Payload < candidates[j].Payload
	})

	// 选择负载最低的节点
	if len(candidates) > 0 {
		return candidates[0], nil
	}

	return nil, errors.New("no worker nodes")
}

/*
无输入，无输出

该方法用于在分布式系统中批量创建种子资源，首先检查etcd中是否存在该资源避免重复添加，接着将不存在的资源批量加入etcd，通过雪花算法为该资源生成唯一id，接着将该资源基于负载均衡算法分配给一个工作节点，在检查节点有效之后记录节点分配信息，并持久化到etcd中，最后将数据同步到内存中，并更新节点负载
*/
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
	for _, r := range rs {
		m.addResources(r)
	}
}

/*
无输入，输出一个错误

该方法用于从etcd中加载所有资源信息，进行反序列化存储到内存中并更新master资源列表，遍历所有资源，重建节点负载
*/
func (m *Master) loadResource() error {
	// 利用etcd客户端的Get方法从etcd中获取所有资源信息
	resp, err := m.etcdCli.Get(context.Background(), RESOURCEPATH, clientv3.WithSerializable())
	if err != nil {
		return fmt.Errorf("etcd get failed")
	}

	resources := make(map[string]*ResourceSpec)
	// 遍历所有键值对，反序列化并存储
	for _, kv := range resp.Kvs {
		r, err := Decode(kv.Value)
		if err == nil && r != nil {
			resources[r.Name] = r
		}
	}
	m.logger.Info("leader init load resource", zap.Int("lenth", len(m.resources)))
	// 更新资源列表
	m.rlock.Lock()
	defer m.rlock.Unlock()
	m.resources = resources

	// 遍历所有资源
	for _, r := range m.resources {
		// 检查是否有分配节点
		if r.AssignedNode != "" {
			id, err := getNodeID(r.AssignedNode)
			if err != nil {
				m.logger.Error("getNodeID failed", zap.Error(err))
			}
			// 查找对应工作节点
			node, ok := m.workNodes[id]
			if ok {
				node.Payload++ // 增加节点负载计数
			}
		}
	}

	return nil
}

/*
无输入，无输出

该方法用于在分布式系统中重新分配资源，两轮遍历，第一轮遍历所有资源检查尚未分配的资源，加入重分配队列，若已分配资源则检查节点是否存在，不存在也将资源加入重分配队列，第二轮遍历对需要重新分配的资源执行添加资源的方法，通过雪花算法为该资源生成唯一id，接着将该资源基于负载均衡算法分配给一个工作节点，在检查节点有效之后记录节点分配信息，并持久化到etcd中，最后将数据同步到内存中，并更新节点负载
*/
func (m *Master) reAssign() {
	// 创建待重新分配的资源切片
	rs := make([]*ResourceSpec, 0, len(m.resources))

	m.rlock.Lock()
	defer m.rlock.Unlock()

	// 第一轮遍历：筛选需要重新分配的资源
	for _, r := range m.resources {
		// 情况1：资源尚未分配节点
		if r.AssignedNode == "" {
			rs = append(rs, r) // 加入重分配队列
			continue
		}

		// 解析节点id
		id, err := getNodeID(r.AssignedNode)

		if err != nil {
			m.logger.Error("get nodeid failed", zap.Error(err))
		}

		// 情况2：节点不存在
		if _, ok := m.workNodes[id]; !ok {
			rs = append(rs, r)
		}
	}

	// 第二轮遍历：执行重新分配
	for _, r := range rs {
		m.addResources(r)
	}
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

// 用于描述worker节点状态
type NodeSpec struct {
	Node    *registry.Node
	Payload int32
}

// 资源类型定义
type ResourceSpec struct {
	ID           string // 资源id
	Name         string // 资源名称
	AssignedNode string // 格式为"Worker节点ID|节点地址"
	CreationTime int64  // 创建时间
}

/*
输入一个资源规格，输出一个字符串

该方法用于将资源规则序列化为一个字符串
*/
func encode(s *ResourceSpec) string {
	b, _ := json.Marshal(s)
	return string(b)
}

/*
输入一个资源名称，输出一个字符串

该方法用于根据资源名称生成资源在etcd中的存储路径
*/
func getResourcePath(name string) string {
	return fmt.Sprintf("%s/%s", RESOURCEPATH, name)
}

/*
输入一个旧节点集合和一个新节点集合，输出新增节点id切片、删除节点id切片和变更节点id切片

该方法用于对比新旧工作节点信息，创建新增节点切片、删除节点切片、变更节点切片，两轮遍历，第一轮遍历检测新增和变更的节点（节点不在旧集合中视为新节点，在旧集合中则深层比较节点对象看是否发生变更），第二轮遍历检测删除的节点（节点不在新集合中视为删除节点）
*/
func workNodeDiff(old map[string]*NodeSpec, new map[string]*NodeSpec) ([]string, []string, []string) {
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

/*
无输入，输出一个字符串和一个错误

该方法用于获取本机网卡IP地址，遍历所有网卡，找到第一个非lo的网卡IP，若找到则返回该IP地址，否则返回错误
*/
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
/*
输入一个id、一个选项列表，输出一个master实例和一个错误

该方法用于创建一个新的master实例，首先合并配置，初始化内存资源表和雪花算法id生成器，获取主机IP后生成唯一的masterID，连接etcd集群并从中加载worker节点，添加预定义的资源，启动协程进行leader选举
*/
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
	// go m.HandleMsg()    // 启动协程处理消息
	return m, nil
}

/*
输入一个id、一个ipv4地址和一个GRPC地址，输出一个字符串

该方法用于根据给定的id、ipv4地址和GRPC地址生成一个唯一的masterID
*/
func genMasterID(id string, ipv4 string, GRPCAddress string) string {
	return "master" + id + "-" + ipv4 + GRPCAddress
}

/*
输入一个分配节点字符串，输出一个节点id和一个错误

该方法用于从分配节点字符串中提取节点id，若提取失败则返回空字符串和错误
*/
func getNodeID(assigned string) (string, error) {
	node := strings.Split(assigned, "|")
	if len(node) < 2 {
		return "", errors.New("")
	}
	id := node[0]
	return id, nil
}

/*
输入一个json格式的字节切片，输出一个资源规格和一个错误

该方法用于将json格式的字节切片反序列化为一个资源规格
*/
func Decode(ds []byte) (*ResourceSpec, error) {
	var s *ResourceSpec
	err := json.Unmarshal(ds, &s)
	return s, err
}
