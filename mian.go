package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/PuerkitoBio/goquery" // 用于CSS选择器支持
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// 正则表达式中[\s\S]*?是精髓，用于匹配任意字符串，*代表匹配0个或多个字符，?代表贪婪匹配，即找到第一个<a target="_blank"出现的未知就认定匹配成功，若不指定?则会找到最后一个<a target="_blank"
// var headerRe = regexp.MustCompile(`<div class="index_[\s\S]*?<a target="_blank"[\s\S]*?alt="([\s\S]*?)"[\s\S]*?</a>`)

func main() {
	url := "https://www.thepaper.cn/"
	body, err := Fetch(url)
	if err != nil {
		fmt.Printf("read content failed:%v", err)
		return
	}

	// 使用goquery库来解析HTML文档，doc表示整个HTML文档
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		fmt.Printf("read content failed:%v", err)
		return
	}

	// 根据CSS标签选择器的语法查找匹配的标签，并遍历输出a标签中的文本
	// Find方法用于查找匹配指定CSS选择器的元素，即选择属性以index_开头的div标签，^=是属性选择器，表示属性值以指定字符串开头
	// a[target=_blank]选择target属性为_blank的a标签，img[alt]选择有alt属性的img标签
	// each方法用于遍历所有匹配的元素，匿名函数的形式，s表示当前匹配的元素
	doc.Find("div[class^='index_'] a[target=_blank] img[alt]").Each(func(i int, s *goquery.Selection) {
		// 即获取当前img标签的alt属性值
		title := s.AttrOr("alt", "")
		fmt.Printf("Review %d: %s\n", i, title)
	})

	// 用于解析HTML文本
	// doc, err := htmlquery.Parse(bytes.NewReader(body))

	// 通过XPath语法查找符合条件的节点
	// //表示选择文档中的任意位置，选择div标签，要求class属性包含index_字符，选择div标签下的a标签，选择a标签下的img标签，选择img标签下的alt属性并提取
	// nodes := htmlquery.Find(doc, `//div[contains(@class, 'index_')]/a/img/@alt]`)
	//fmt.Println(string(body))
	/*
		for _, node := range nodes {
			fmt.Println("fetch card ", node.FirstChild.Data)
		}
	*/
	// 返回表达式的所有连续匹配的切片，是一个三维字节数组，第一层包含的是所有满足正则条件的字符串，第二层对每个满足条件的字符串做了分组，数组的第0号元素是<a>标签中的文字即新闻标题，第三层是字符串对应的字节数组
	/*
		matches := headerRe.FindAllSubmatch(body, -1)
		for _, m := range matches {
			fmt.Println("fetch card news:", string(m[1]))
		}
	*/
}

// 用于获取指定URL的HTML文本并转换为UTF-8编码方式
func Fetch(url string) ([]byte, error) {

	resp, err := http.Get(url)

	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Error status code:%d", resp.StatusCode)
	}
	bodyReader := bufio.NewReader(resp.Body)
	e := DeterminEncoding(bodyReader)
	// 将HTML文本从特定编码转换为UTF-8编码
	utf8Reader := transform.NewReader(bodyReader, e.NewDecoder())
	return io.ReadAll(utf8Reader)
}

// 用于检测并返回当前HTML文本的编码格式
func DeterminEncoding(r *bufio.Reader) encoding.Encoding {
	// 当返回的HTML文本小于1024字节，则认为当前HTML文本有问题，直接返回默认的UTF-8编码
	bytes, err := r.Peek(1024)

	if err != nil {
		fmt.Printf("fetch error:%v", err)
		return unicode.UTF8
	}
	// 检测并返回对应HTML文本的编码
	e, _, _ := charset.DetermineEncoding(bytes, "")
	return e
}
