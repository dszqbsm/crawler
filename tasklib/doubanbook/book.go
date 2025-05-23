package doubanbook

// 实现对豆瓣书籍信息的抓取和解析功能，首先获取热门标签信息，然后获取不同标签下的热门书籍列表，最后获取书籍信息并存储到数据库中

import (
	"regexp"
	"strconv"
	"time"

	"github.com/dszqbsm/crawler/limiter"
	"github.com/dszqbsm/crawler/spider"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

var DoubanBookTask = &spider.Task{
	Options: spider.Options{
		Name: "douban_book_list",
		Limit: limiter.Multi(
			rate.NewLimiter(limiter.Per(1, 3*time.Second), 1),
			rate.NewLimiter(limiter.Per(20, 60*time.Second), 20),
		),
		Cookie:   "bid=-UXUw--yL5g; push_doumail_num=0; __utmv=30149280.21428; __utmc=30149280; __gads=ID=c6eaa3cb04d5733a-2259490c18d700e1:T=1666111347:RT=1666111347:S=ALNI_MaonVB4VhlZG_Jt25QAgq-17DGDfw; frodotk_db=\"17dfad2f83084953479f078e8918dbf9\"; gr_user_id=cecf9a7f-2a69-4dfd-8514-343ca5c61fb7; __utmc=81379588; _vwo_uuid_v2=D55C74107BD58A95BEAED8D4E5B300035|b51e2076f12dc7b2c24da50b77ab3ffe; __yadk_uid=BKBuETKRjc2fmw3QZuSw4rigUGsRR4wV; ll=\"108288\"; viewed=\"36104107\"; push_noty_num=0; __gpi=UID=000008887412003e:T=1666111347:RT=1671298423:S=ALNI_MZmNsuRnBrad4_ynFUhTl0Hi0l5oA; ap_v=0,6.0; __utma=30149280.2072705865.1665849857.1671552282.1672405255.38; __utmz=30149280.1672405255.38.7.utmcsr=sec.douban.com|utmccn=(referral)|utmcmd=referral|utmcct=/; __utmt=1; dbcl2=\"214281202:I/fgB5VGk7w\"; ck=X5WM; __utmt_douban=1; __utmb=30149280.2.10.1672405255; __utma=81379588.990530987.1667661846.1671552282.1672405445.18; __utmz=81379588.1672405445.18.9.utmcsr=accounts.douban.com|utmccn=(referral)|utmcmd=referral|utmcct=/; __utmb=81379588.1.10.1672405445; _pk_ref.100001.3ac3=[\"\",\"\",1672405446,\"https://accounts.douban.com/\"]; _pk_id.100001.3ac3=02339dd9cc7d293a.1667661846.18.1672405446.1671552282.; _pk_ses.100001.3ac3=*; gr_session_id_22c937bbd8ebd703f2d8e9445f7dfd03=11d21514-2891-468d-ac0b-cf08a1a6085d; gr_cs1_11d21514-2891-468d-ac0b-cf08a1a6085d=user_id:1; gr_session_id_22c937bbd8ebd703f2d8e9445f7dfd03_11d21514-2891-468d-ac0b-cf08a1a6085d=true",
		Reload:   true,
		WaitTime: 2,
		MaxDepth: 5,
	},
	// 根任务规则
	Rule: spider.RuleTree{
		Root: func() ([]*spider.Request, error) {
			roots := []*spider.Request{
				{
					Priority: 1,
					Url:      "https://book.douban.com",
					Method:   "GET",
					RuleName: "数据tag",
				},
			}
			return roots, nil
		},
		// 子任务规则
		Trunk: map[string]*spider.Rule{
			"数据tag": {ParseFunc: ParseTag},
			"书籍列表":  {ParseFunc: ParseBookList},
			"书籍简介": {
				ItemFields: []string{
					"书名",
					"作者",
					"页数",
					"出版社",
					"得分",
					"价格",
					"简介",
				},
				ParseFunc: ParseBookDetail,
			},
		},
	},
}

// 解析页面中的标签链接，提取url生成新的请求
const regexpStr = `<a href="([^"]+)" class="tag">([^<]+)</a>`

func ParseTag(ctx *spider.Context) (spider.ParseResult, error) {
	re := regexp.MustCompile(regexpStr)

	matches := re.FindAllSubmatch(ctx.Body, -1)
	result := spider.ParseResult{}

	for _, m := range matches {
		result.Requests = append(
			result.Requests, &spider.Request{
				Method:   "GET",
				Task:     ctx.Req.Task,
				Url:      "https://book.douban.com" + string(m[1]),
				Depth:    ctx.Req.Depth + 1,
				RuleName: "书籍列表",
			})
	}
	zap.S().Debugln("parse book tag,count:", len(result.Requests), "url:", ctx.Req.Url)
	// 在添加limit之前，临时减少抓取数量,防止被服务器封禁
	// result.Requests = result.Requests[:1]
	return result, nil
}

// 解析书籍列表页面，提取书籍的url和名称，生成书籍详情页面的请求
const BooklistRe = `<a.*?href="([^"]+)" title="([^"]+)"`

func ParseBookList(ctx *spider.Context) (spider.ParseResult, error) {
	re := regexp.MustCompile(BooklistRe)
	matches := re.FindAllSubmatch(ctx.Body, -1)
	result := spider.ParseResult{}
	for _, m := range matches {
		req := &spider.Request{
			Priority: 100,
			Method:   "GET",
			Task:     ctx.Req.Task,
			Url:      string(m[1]),
			Depth:    ctx.Req.Depth + 1,
			RuleName: "书籍简介",
		}
		req.TmpData = &spider.Temp{}
		// req.TmpData.Set("book_name", string(m[2]))
		if err := req.TmpData.Set("book_name", string(m[2])); err != nil {
			zap.L().Error("Set TmpData failed", zap.Error(err))
		}
		result.Requests = append(result.Requests, req)
	}
	// 在添加limit之前，临时减少抓取数量,防止被服务器封禁
	// result.Requests = result.Requests[:3]
	zap.S().Debugln("parse book list,count:", len(result.Requests), "url:", ctx.Req.Url)

	return result, nil
}

// 解析书籍详情页面，提取书籍的详细信息，并将其存储到数据单元中
var autoRe = regexp.MustCompile(`<span class="pl"> 作者</span>:[\d\D]*?<a.*?>([^<]+)</a>`)
var public = regexp.MustCompile(`<span class="pl">出版社:</span>([^<]+)<br/>`)
var pageRe = regexp.MustCompile(`<span class="pl">页数:</span> ([^<]+)<br/>`)
var priceRe = regexp.MustCompile(`<span class="pl">定价:</span>([^<]+)<br/>`)
var scoreRe = regexp.MustCompile(`<strong class="ll rating_num " property="v:average">([^<]+)</strong>`)
var intoRe = regexp.MustCompile(`<div class="intro">[\d\D]*?<p>([^<]+)</p></div>`)

func ParseBookDetail(ctx *spider.Context) (spider.ParseResult, error) {
	bookName := ctx.Req.TmpData.Get("book_name")
	page, _ := strconv.Atoi(ExtraString(ctx.Body, pageRe))

	book := map[string]interface{}{
		"书名":  bookName,
		"作者":  ExtraString(ctx.Body, autoRe),
		"页数":  page,
		"出版社": ExtraString(ctx.Body, public),
		"得分":  ExtraString(ctx.Body, scoreRe),
		"价格":  ExtraString(ctx.Body, priceRe),
		"简介":  ExtraString(ctx.Body, intoRe),
	}
	data := ctx.Output(book)

	result := spider.ParseResult{
		Items: []interface{}{data},
	}
	zap.S().Debugln("parse book detail", data)

	return result, nil
}

// 从给定的内容中提取匹配正则表达式的字符串
func ExtraString(contents []byte, re *regexp.Regexp) string {

	match := re.FindSubmatch(contents)

	if len(match) >= 2 {
		return string(match[1])
	} else {
		return ""
	}
}
