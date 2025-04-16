package spider

import (
	"context"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

type EventType int

const (
	EventTypeDelete EventType = iota
	EventTypePut

	RESOURCEPATH = "/resources"
)

type WatchResponse struct {
	Typ      EventType
	Res      *ResourceSpec
	Canceled bool
}

type WatchChan chan WatchResponse

type ResourceRegistry interface {
	/*
	   无输入，输出一个资源列表和一个错误

	   该方法用于获取etcd中存储的所有资源，并返回资源列表和错误
	*/
	GetResources() ([]*ResourceSpec, error)
	/*
	   无输入，输出一个WatchChan类型的通道

	   该方法用于监听etcd中/resources 路径下的资源变化，并通过 WatchChan 通道异步推送事件详情

	   首先初始化etcd Watch，for循环持续监听事件流，当有事件流来时，首先处理错误和中断，然后处理被取消的情况，最后循环遍历所有事件，根据事件类型处理资源的创建、更新和删除

	   对于资源创建和更新，首先将资源解码，区分创建和更新，推送事件到 WatchChan 通道

	   对于资源删除，首先将资源解码，推送事件到 WatchChan 通道
	*/
	WatchResources() WatchChan
}

type EtcdRegistry struct {
	etcdCli *clientv3.Client
}

/*
输入一个etcd的地址列表，输出一个EtcdRegistry实例和一个错误

该方法用于创建一个EtcdRegistry实例，连接到指定的etcd服务器，并返回实例和错误
*/
func NewEtcdRegistry(endpoints []string) (ResourceRegistry, error) {
	cli, err := clientv3.New(clientv3.Config{Endpoints: endpoints})
	return &EtcdRegistry{cli}, err
}

/*
无输入，输出一个资源列表和一个错误

该方法用于获取etcd中存储的所有资源，并返回资源列表和错误
*/
func (e *EtcdRegistry) GetResources() ([]*ResourceSpec, error) {
	resp, err := e.etcdCli.Get(context.Background(), RESOURCEPATH, clientv3.WithPrefix(), clientv3.WithSerializable())
	if err != nil {
		return nil, err
	}
	resources := make([]*ResourceSpec, 0)
	for _, kv := range resp.Kvs {
		r, err := Decode(kv.Value)
		if err == nil && r != nil {
			resources = append(resources, r)
		}
	}
	return resources, nil
}

/*
无输入，输出一个WatchChan类型的通道

该方法用于监听etcd中/resources 路径下的资源变化，并通过 WatchChan 通道异步推送事件详情

首先初始化etcd Watch，for循环持续监听事件流，当有事件流来时，首先处理错误和中断，然后处理被取消的情况，最后循环遍历所有事件，根据事件类型处理资源的创建、更新和删除

对于资源创建和更新，首先将资源解码，区分创建和更新，推送事件到 WatchChan 通道

对于资源删除，首先将资源解码，推送事件到 WatchChan 通道
*/
func (e *EtcdRegistry) WatchResources() WatchChan {
	ch := make(WatchChan)
	go func() {
		// WithPrefix监听前缀为RESOURCEPATH的所有键，WithPrevKV在删除事件中携带被删除资源的旧值，用于获取被删除资源详情
		watch := e.etcdCli.Watch(context.Background(), RESOURCEPATH, clientv3.WithPrefix(), clientv3.WithPrevKV())
		for w := range watch {
			// 处理错误和中断
			if w.Err() != nil {
				zap.S().Error("watch resource failed", zap.Error(w.Err()))
				continue
			}
			// 监听被取消，如etcd断开
			if w.Canceled {
				zap.S().Error("watch resource canceled")
				ch <- WatchResponse{
					Canceled: true,
				}
			}
			for _, ev := range w.Events {

				switch ev.Type {
				// 资源创建和更新
				case clientv3.EventTypePut:
					spec, err := Decode(ev.Kv.Value)
					if err != nil {
						zap.S().Error("decode etcd value failed", zap.Error(err))
					}
					if ev.IsCreate() {
						zap.S().Info("receive create resource", zap.Any("spec", spec))
					} else if ev.IsModify() {
						zap.S().Info("receive update resource", zap.Any("spec", spec))
					}

					ch <- WatchResponse{
						EventTypePut,
						spec,
						false,
					}
					// 资源删除
				case clientv3.EventTypeDelete:
					spec, err := Decode(ev.PrevKv.Value)
					zap.S().Info("receive delete resource", zap.Any("spec", spec))
					if err != nil {
						zap.S().Error("decode etcd value failed", zap.Error(err))
					}
					ch <- WatchResponse{
						EventTypeDelete,
						spec,
						false,
					}
				}
			}
		}
	}()

	return ch

}
