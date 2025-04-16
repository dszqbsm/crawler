package doubangroup

// 基于静态规则爬取豆瓣组内包含阳台关键字的页面

import (
	"fmt"
	"regexp"

	"github.com/dszqbsm/crawler/spider"
)

// 匹配豆瓣小组话题的url
const urlListRe = `(https://www.douban.com/group/topic/[0-9a-z]+/)"[^>]*>([^<]+)</a>`

// 匹配包含阳台关键字的页面内容
const ContentRe = `<div class="topic-content">[\s\S]*?阳台[\s\S]*?<div class="aside">`

/*
初始化豆瓣组任务，定义根任务规则和子任务规则

根任务规则：从豆瓣组某个话题的首页开始，因为每页只有25个话题，因此提取前四页的网站url，生成了根请求

子任务规则：

- 解析网站URL：解析根请求的响应，基于正则表达式从响应结果中匹配当前页面中的话题url，生成子请求

- 解析阳台房：解析子请求的响应，基于正则表达式从响应结果中匹配页面内容含有”阳台房“字样的url
*/
var DoubangroupTask = &spider.Task{
	Rule: spider.RuleTree{
		Root: func() ([]*spider.Request, error) {
			var roots []*spider.Request
			for i := 0; i < 25; i += 25 {
				str := fmt.Sprintf("https://www.douban.com/group/szsh/discussion?start=%d", i)
				roots = append(roots, &spider.Request{
					Priority: 1,
					Url:      str,
					Method:   "GET",
					RuleName: "解析网站URL",
				})
			}
			return roots, nil
		},
		Trunk: map[string]*spider.Rule{
			"解析网站URL": {ParseFunc: ParseURL},
			"解析阳台房":   {ParseFunc: GetSunRoom},
		},
	},
}

/*
输入一个响应结果结构体，输出一个解析结果结构体和一个错误

该函数作为一个子规则，用于解析网站url，即基于正则表达式从响应结果中匹配当前页面中的话题url，生成子请求
*/
func ParseURL(ctx *spider.Context) (spider.ParseResult, error) {
	re := regexp.MustCompile(urlListRe)

	matches := re.FindAllSubmatch(ctx.Body, -1)
	result := spider.ParseResult{}

	for _, m := range matches {
		u := string(m[1])
		result.Requests = append(
			result.Requests, &spider.Request{
				Method:   "GET",
				Task:     ctx.Req.Task,
				Url:      u,
				Depth:    ctx.Req.Depth + 1,
				RuleName: "解析阳台房",
			})
	}
	return result, nil
}

func GetSunRoom(ctx *spider.Context) (spider.ParseResult, error) {
	re := regexp.MustCompile(ContentRe)

	ok := re.Match(ctx.Body)
	if !ok {
		return spider.ParseResult{
			Items: []interface{}{},
		}, nil
	}
	result := spider.ParseResult{
		Items: []interface{}{ctx.Req.Url},
	}
	return result, nil
}
