package doubangroup

import (
	"fmt"
	"regexp"

	"github.com/dszqbsm/crawler/collect"
)

// 获取当前页面中所有话题url
const urlListRe = `href="(https://www\.douban\.com/group/topic/\d+/)[^"]*"`

func ParseURL(contents []byte, req *collect.Request) collect.ParseResult {
	re := regexp.MustCompile(urlListRe)

	// re.FindAllSubmatch返回一个二维切片，外层切片表示所有匹配的结果，内层切片表示每个匹配结果中的捕获组（还分为m[0]表示整个匹配的内容，m[1]表示第一个捕获组，m[2]表示第二个捕获组）
	matches := re.FindAllSubmatch(contents, -1)
	result := collect.ParseResult{}
	// fmt.Println("来到ParseURL函数")
	// fmt.Println("匹配到的matches为", matches)
	// m表示单个匹配结果的所有捕获组，因此这里使用m[1]表示第一个捕获组，即匹配到的url
	for _, m := range matches {
		u := string(m[1])
		// fmt.Println("匹配到第一个捕获组为", string(m[1]))
		result.Requests = append(
			result.Requests, &collect.Request{
				Url:    u,
				Cookie: req.Cookie,
				ParseFunc: func(c []byte, request *collect.Request) collect.ParseResult {
					return GetContent(c, u)
				},
			})
	}
	// fmt.Printf("解析任务完成 %+v\n", result)
	return result
}

// 过滤当前话题正文中存在阳台字样的话题url
const ContentRe = `<div class="topic-content">[\s\S]*?阳台[\s\S]*?<div`

func GetContent(contents []byte, url string) collect.ParseResult {
	re := regexp.MustCompile(ContentRe)

	ok := re.Match(contents)
	if !ok {
		fmt.Println("阳台字样不存在")
		return collect.ParseResult{
			// 初始化为空接口切片为空，因此有两个花括号
			Items: []interface{}{},
		}
	}

	fmt.Println("阳台字样存在", url)
	result := collect.ParseResult{
		Items: []interface{}{url},
	}

	return result
}
