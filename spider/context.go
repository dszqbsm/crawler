package spider

import (
	"regexp"
	"time"
)

type Context struct {
	Body []byte
	Req  *Request
}

/*
输入一个数据，输出一个数据单元

该方法用于创建一个数据单元，将数据和请求的相关信息封装到数据单元中，返回数据单元的指针
*/
func (c *Context) Output(data interface{}) *DataCell {
	res := &DataCell{
		Task: c.Req.Task,
	}
	res.Data = make(map[string]interface{})
	res.Data["Task"] = c.Req.Task.Name
	res.Data["Rule"] = c.Req.RuleName
	res.Data["Data"] = data
	res.Data["URL"] = c.Req.Url
	res.Data["Time"] = time.Now().Format("2006-01-02 15:04:05")

	return res
}

/*
输入一个规则名称和正则表达式，输出解析结果

该方法用于解析网页内容，根据正则表达式提取网页中的链接，并将链接封装成请求，添加到解析结果的请求列表中，返回解析结果
*/
func (c *Context) ParseJSReg(name string, reg string) ParseResult {
	re := regexp.MustCompile(reg)

	matches := re.FindAllSubmatch(c.Body, -1)
	result := ParseResult{}

	for _, m := range matches {
		u := string(m[1])

		result.Requests = append(
			result.Requests, &Request{
				Method:   "GET",
				Task:     c.Req.Task,
				Url:      u,
				Depth:    c.Req.Depth + 1,
				RuleName: name,
			})
	}

	return result
}

/*
输入一个正则表达式，输出解析结果

该方法用于解析网页内容，根据正则表达式匹配响应内容，若匹配成功则记录当前请求的 URL，否则返回空结果
*/
func (c *Context) OutputJS(reg string) ParseResult {
	re := regexp.MustCompile(reg)
	if ok := re.Match(c.Body); !ok {
		return ParseResult{
			Items: []interface{}{},
		}
	}

	result := ParseResult{
		Items: []interface{}{c.Req.Url},
	}

	return result
}
