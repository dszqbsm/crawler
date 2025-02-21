// 采集引擎

package collect

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/dszqbsm/crawler/extensions"
	"github.com/dszqbsm/crawler/proxy"
	"go.uber.org/zap"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// Fetcher接口用于统一不同采集器的实现
type Fetcher interface {
	Get(url *Request) ([]byte, error)
}

// 基本的爬取
type BaseFetch struct {
}

// 实现Fetcher接口，基本的爬取：请求、读响应、检测编码、转utf-8编码，http.Get封装了创建client、创建NewRequest，调用Do方法的过程
func (BaseFetch) Get(req *Request) ([]byte, error) {
	resp, err := http.Get(req.Url)

	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Error status code:%d\n", resp.StatusCode)
		return nil, err
	}
	bodyReader := bufio.NewReader(resp.Body)
	e := DeterminEncoding(bodyReader)
	utf8Reader := transform.NewReader(bodyReader, e.NewDecoder())
	return ioutil.ReadAll(utf8Reader)
}

// 模拟浏览器爬取，只是加一个user-agent头
type BrowserFetch struct {
	Timeout time.Duration
	Proxy   proxy.ProxyFunc
	Logger  *zap.Logger
}

// 实现Fetcher接口，模拟浏览器爬取：为了设置HTTP请求头，不能再简单使用http.Get方法，需要创建http.Client，并通过http.NewRequest创建一个请求，并调用req.Header.Set设置User-Agent头，调用client.Do完成http请求
// 创建http.Client，http.NewRequest创建一个请求，req.Header.Set设置User-Agent头，client.Do完成http请求，读响应，检测编码，转utf-8
func (b BrowserFetch) Get(request *Request) ([]byte, error) {
	client := &http.Client{
		Timeout: b.Timeout,
	}

	// 将默认的代理地址选择函数替换成自定义的轮询代理地址选择函数
	if b.Proxy != nil {
		transport := http.DefaultTransport.(*http.Transport)
		transport.Proxy = b.Proxy
		client.Transport = transport
	}

	req, err := http.NewRequest("GET", request.Url, nil)
	if err != nil {
		return nil, fmt.Errorf("get url failed:%v", err)
	}

	if len(request.Task.Cookie) > 0 {
		req.Header.Set("Cookie", request.Task.Cookie)
	}

	req.Header.Set("User-Agent", extensions.GenerateRandomUA())

	resp, err := client.Do(req)

	time.Sleep(request.Task.WaitTime)

	if err != nil {
		b.Logger.Error("fetch failed",
			zap.Error(err),
		)
		return nil, err
	}

	bodyReader := bufio.NewReader(resp.Body)
	e := DeterminEncoding(bodyReader)
	utf8Reader := transform.NewReader(bodyReader, e.NewDecoder())
	return ioutil.ReadAll(utf8Reader)
}

// 用于检测输入流的字符编码
func DeterminEncoding(r *bufio.Reader) encoding.Encoding {

	bytes, err := r.Peek(1024)

	if err != nil {
		fmt.Printf("fetch error:%v\n", err)
		return unicode.UTF8
	}

	e, _, _ := charset.DetermineEncoding(bytes, "")
	return e
}
