package proxy

import (
	"errors"
	"net/http"
	"net/url"
	"sync/atomic"
)

type ProxyFunc func(*http.Request) (*url.URL, error)

type roundRobinSwitcher struct {
	proxyURLs []*url.URL
	index     uint32
}

/*
输入一个http.Request，输出一个url.URL和一个error。

该方法用于实现代理服务器的轮询调度，它会根据当前的轮询索引选择一个代理服务器，并返回该服务器的URL。
*/
func (r *roundRobinSwitcher) GetProxy(pr *http.Request) (*url.URL, error) {
	if len(r.proxyURLs) == 0 {
		return nil, errors.New("empty proxy urls")
	}
	index := atomic.AddUint32(&r.index, 1) - 1
	u := r.proxyURLs[index%uint32(len(r.proxyURLs))]
	return u, nil
}

/*
输入一个代理服务器地址列表，输出一个代理服务器切换函数和一个error。

该方法用于创建一个轮询调度的代理服务器切换函数，它会根据传入的代理服务器地址列表，创建一个轮询调度器，并返回一个代理服务器切换函数
*/
func RoundRobinProxySwitcher(ProxyURLs ...string) (ProxyFunc, error) {
	if len(ProxyURLs) < 1 {
		return nil, errors.New("Proxy URL list is empty")
	}
	urls := make([]*url.URL, len(ProxyURLs))
	for i, u := range ProxyURLs {
		parsedU, err := url.Parse(u)
		if err != nil {
			return nil, err
		}
		urls[i] = parsedU
	}
	return (&roundRobinSwitcher{urls, 0}).GetProxy, nil
}
