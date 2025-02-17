package main

import (
	"fmt"
	"time"

	"github.com/dszqbsm/crawler/collect"
	"github.com/dszqbsm/crawler/engine"
	"github.com/dszqbsm/crawler/log"
	"github.com/dszqbsm/crawler/parse/doubangroup"
	"go.uber.org/zap/zapcore"
)

func main() {
	// log
	plugin := log.NewStdoutPlugin(zapcore.InfoLevel)
	logger := log.NewLogger(plugin)
	logger.Info("log init end")

	// proxy
	/* 	proxyURLs := []string{"http://127.0.0.1:8888", "http://127.0.0.1:8888"}
	   	// 将代理服务器地址列表解析成url.URL切片，并返回轮询选择函数
	   	p, err := proxy.RoundRobinProxySwitcher(proxyURLs...)
	   	if err != nil {
	   		logger.Error("RoundRobinProxySwitcher failed")
	   	} */

	// douban cookie
	var seeds = make([]*collect.Request, 0, 1000)
	for i := 0; i <= 100; i += 25 {
		str := fmt.Sprintf("https://www.douban.com/group/szsh/discussion?start=%d", i)
		seeds = append(seeds, &collect.Request{
			Url:       str,
			WaitTime:  1 * time.Second,
			Cookie:    "bid=GT7j6PWkiMk; __utmz=30149280.1728978779.1.1.utmcsr=google|utmccn=(organic)|utmcmd=organic|utmctr=(not%20provided); viewed=\"1007305_27055717_10519268_36558411\"; _vwo_uuid_v2=D67C36ABC33CA10C09168D0521AD9DEA5|757429c706db3b67694302a45aec0c8c; _pk_id.100001.8cb4=e13fcef95dbf1252.1739513080.; _pk_ses.100001.8cb4=1; ap_v=0,6.0; __utma=30149280.212206049.1728978779.1738554951.1739513081.5; __utmc=30149280; __utmt=1; __yadk_uid=ZLSVCuyJ41KbJVbVBgV5zd3bnFyY9HhZ; douban-fav-remind=1; dbcl2=\"264055423:qy57Xf9rC78\"; ck=q-ZJ; push_noty_num=0; push_doumail_num=0; __utmv=30149280.26405; __utmb=30149280.126.7.1739513625676",
			ParseFunc: doubangroup.ParseURL,
		})
	}

	var f collect.Fetcher = &collect.BrowserFetch{
		Timeout: 3000 * time.Millisecond,
		Logger:  logger,
		Proxy:   nil,
	}

	s := engine.NewSchedule(
		engine.WithFetcher(f),
		engine.WithLogger(logger),
		engine.WithWorkCount(5),
		engine.WithSeeds(seeds),
	)

	s.Run()
	// 这里将广度优先思想在任务调度那里实现了，通过通道不断向队列中添加任务
	/* 	for len(worklist) > 0 {
		items := worklist
		// 一个worklist就是一个结构体切片，这里表示将当前worklist清空，确保当前批次的任务被完全处理完再将新的任务添加到worklist中
		worklist = nil
		// 会进行两层广度优先，第一层爬取所有话题url，第二层过滤正文中包含阳台字样的话题url，通过解析函数的改变可以实现这样的功能替换
		for _, item := range items {
			body, err := f.Get(item)
			time.Sleep(1 * time.Second)
			if err != nil {
				logger.Error("read content failed",
					zap.Error(err),
				)
				continue
			}
			res := item.ParseFunc(body, item)
			for _, item := range res.Items {
				logger.Info("result",
					zap.String("get url:", item.(string)))
			}
			worklist = append(worklist, res.Requesrts...)
		}
	} */
}
