package doubanbook

// 实现对豆瓣书籍信息的抓取和解析功能，首先获取热门标签信息，然后获取不同标签下的热门书籍列表，最后获取书籍信息并存储到数据库中

import (
	"regexp"
	"strconv"

	"github.com/dszqbsm/crawler/spider"
	"go.uber.org/zap"
)

var DoubanBookTask = &spider.Task{
	/* 	Property: spider.Property{
		Name:     "douban_book_list",
		WaitTime: 2,
		MaxDepth: 5,
		Cookie:   "bid=GT7j6PWkiMk; _pk_id.100001.3ac3=990ea61bddf62bb7.1727589184.; __yadk_uid=qrCDkWQlfaIvB03UawN9VuHC9GXXURbB; __utmz=30149280.1728978779.1.1.utmcsr=google|utmccn=(organic)|utmcmd=organic|utmctr=(not%20provided); __utmz=81379588.1728978779.1.1.utmcsr=google|utmccn=(organic)|utmcmd=organic|utmctr=(not%20provided); _vwo_uuid_v2=D67C36ABC33CA10C09168D0521AD9DEA5|757429c706db3b67694302a45aec0c8c; viewed=\"1007305_27055717_10519268_36558411\"; _vwo_uuid_v2=D67C36ABC33CA10C09168D0521AD9DEA5|757429c706db3b67694302a45aec0c8c; douban-fav-remind=1; dbcl2=\"264055423:qy57Xf9rC78\"; push_noty_num=0; push_doumail_num=0; __utmv=30149280.26405; ck=q-ZJ; __utmc=30149280; __utma=30149280.212206049.1728978779.1740022125.1740104072.14; __utmt=1; _pk_ref.100001.3ac3=%5B%22%22%2C%22%22%2C1740104076%2C%22https%3A%2F%2Fwww.google.com%2F%22%5D; _pk_ses.100001.3ac3=1; ap_v=0,6.0; __utmt_douban=1; __utmb=30149280.8.5.1740104072; __utma=81379588.1610336888.1728978779.1738554951.1740104076.5; __utmc=81379588; __utmb=81379588.1.10.1740104076",
	}, */
	Options: spider.Options{Name: "douban_book_list"},
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
	zap.S().Debugln("parse book tag,count:", len(result.Requests))
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
	zap.S().Debugln("parse book list,count:", len(result.Requests))

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
