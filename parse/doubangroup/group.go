package doubangroup

// 基于静态规则爬取豆瓣组内包含阳台关键字的页面

import (
	"fmt"
	"regexp"
	"time"

	"github.com/dszqbsm/crawler/collect"
)

// 匹配豆瓣小组话题的url
const urlListRe = `(https://www.douban.com/group/topic/[0-9a-z]+/)"[^>]*>([^<]+)</a>`

// 匹配包含阳台关键字的页面内容
const ContentRe = `<div class="topic-content">[\s\S]*?阳台[\s\S]*?<div class="aside">`

// 初始化豆瓣组爬虫任务
var DoubangroupTask = &collect.Task{
	Property: collect.Property{
		Name:     "find_douban_sun_room",
		WaitTime: 1 * time.Second,
		MaxDepth: 5,
		Cookie:   "bid=GT7j6PWkiMk; __utmz=30149280.1728978779.1.1.utmcsr=google|utmccn=(organic)|utmcmd=organic|utmctr=(not%20provided); viewed=\"1007305_27055717_10519268_36558411\"; _vwo_uuid_v2=D67C36ABC33CA10C09168D0521AD9DEA5|757429c706db3b67694302a45aec0c8c; _pk_id.100001.8cb4=e13fcef95dbf1252.1739513080.; __yadk_uid=ZLSVCuyJ41KbJVbVBgV5zd3bnFyY9HhZ; douban-fav-remind=1; dbcl2=\"264055423:qy57Xf9rC78\"; push_noty_num=0; push_doumail_num=0; __utmv=30149280.26405; ck=q-ZJ; __utmc=30149280; _pk_ses.100001.8cb4=1; __utma=30149280.212206049.1728978779.1739853169.1740022125.13; __utmt=1; __utmb=30149280.7.5.1740022125",
	},
	Rule: collect.RuleTree{
		Root: func() ([]*collect.Request, error) {
			var roots []*collect.Request
			for i := 0; i < 25; i += 25 {
				str := fmt.Sprintf("https://www.douban.com/group/szsh/discussion?start=%d", i)
				roots = append(roots, &collect.Request{
					Priority: 1,
					Url:      str,
					Method:   "GET",
					RuleName: "解析网站URL",
				})
			}
			return roots, nil
		},
		Trunk: map[string]*collect.Rule{
			"解析网站URL": &collect.Rule{ParseFunc: ParseURL},
			"解析阳台房":   &collect.Rule{ParseFunc: GetSunRoom},
		},
	},
}

func ParseURL(ctx *collect.Context) (collect.ParseResult, error) {
	re := regexp.MustCompile(urlListRe)

	matches := re.FindAllSubmatch(ctx.Body, -1)
	result := collect.ParseResult{}

	for _, m := range matches {
		u := string(m[1])
		result.Requests = append(
			result.Requests, &collect.Request{
				Method:   "GET",
				Task:     ctx.Req.Task,
				Url:      u,
				Depth:    ctx.Req.Depth + 1,
				RuleName: "解析阳台房",
			})
	}
	return result, nil
}

func GetSunRoom(ctx *collect.Context) (collect.ParseResult, error) {
	re := regexp.MustCompile(ContentRe)

	ok := re.Match(ctx.Body)
	if !ok {
		return collect.ParseResult{
			Items: []interface{}{},
		}, nil
	}
	result := collect.ParseResult{
		Items: []interface{}{ctx.Req.Url},
	}
	return result, nil
}
