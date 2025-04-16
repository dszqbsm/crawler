package spider

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"

	"github.com/dszqbsm/crawler/extensions"
	"go.uber.org/zap"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

type FetchType int

const (
	BaseFetchType FetchType = iota
	BrowserFetchType
)

type Fetcher interface {
	/*
	   输入一个请求，输出一个字节数组和一个错误

	   该方法用于发送模拟人类行为的get请求，基于令牌桶算法进行限流，进行随机休眠，设置代理服务器，设置随机User-Agent和Cookie，编码检测并转换为utf-8
	*/
	Get(url *Request) ([]byte, error)
}

/*
输入一个FetchType类型的参数，输出一个Fetcher接口类型的实例

该方法用于创建一个Fetcher接口类型的实例，根据输入的FetchType类型参数选择不同的实现方式，返回对应的Fetcher接口类型的实例
*/
func NewFetchService(typ FetchType) Fetcher {
	switch typ {
	case BaseFetchType:
		return &baseFetch{}
	case BrowserFetchType:
		return &browserFetch{}
	default:
		return &browserFetch{}
	}
}

type baseFetch struct{}

/*
输入一个请求，输出一个字节数组和一个错误

该方法用于发送HTTP GET请求并获取响应，若响应状态码不为200，则返回错误，否则将响应体转换为UTF-8编码，并返回响应体的字节数组和nil错误
*/
func (*baseFetch) Get(req *Request) ([]byte, error) {
	resp, err := http.Get(req.Url)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error status code:%d", resp.StatusCode)
	}

	bodyReader := bufio.NewReader(resp.Body)
	e := DeterminEncoding(bodyReader)
	utf8Reader := transform.NewReader(bodyReader, e.NewDecoder())

	return ioutil.ReadAll(utf8Reader)
}

type browserFetch struct{}

/*
输入一个请求，输出一个字节数组和一个错误

该方法用于发送模拟人类行为的get请求，基于令牌桶算法进行限流，进行随机休眠，设置代理服务器，设置随机User-Agent和Cookie，编码检测并转换为utf-8
*/
func (b *browserFetch) Get(request *Request) ([]byte, error) {
	task := request.Task
	if err := task.Limit.Wait(context.Background()); err != nil {
		return nil, err
	}
	// 随机休眠，模拟人类行为
	sleeptime := rand.Int63n(task.WaitTime * 1000)
	time.Sleep(time.Duration(sleeptime) * time.Millisecond)

	client := &http.Client{
		Timeout: task.Timeout,
	}

	if task.Proxy != nil {
		transport := http.DefaultTransport.(*http.Transport)
		transport.Proxy = task.Proxy
		client.Transport = transport
	}

	req, err := http.NewRequest("GET", request.Url, nil)

	if err != nil {
		return nil, fmt.Errorf("get url failed:%w", err)
	}

	if len(task.Cookie) > 0 {
		req.Header.Set("Cookie", task.Cookie)
	}

	req.Header.Set("User-Agent", extensions.GenerateRandomUA())

	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}

	bodyReader := bufio.NewReader(resp.Body)
	e := DeterminEncoding(bodyReader)
	utf8Reader := transform.NewReader(bodyReader, e.NewDecoder())

	return ioutil.ReadAll(utf8Reader)
}

func DeterminEncoding(r *bufio.Reader) encoding.Encoding {
	bytes, err := r.Peek(1024)

	if err != nil {
		zap.L().Error("fetch failed", zap.Error(err))

		return unicode.UTF8
	}

	e, _, _ := charset.DetermineEncoding(bytes, "")

	return e
}
