package doubangroupjs

// 基于动态规则爬取豆瓣组内包含阳台关键字的页面

import (
	"github.com/dszqbsm/crawler/spider"
)

// 豆瓣组爬虫任务模板
var DoubangroupJSTask = &spider.TaskModle{
	Property: spider.Property{
		Name:     "js_find_douban_sun_room",
		WaitTime: 2,
		MaxDepth: 5,
		Cookie:   "bid=GT7j6PWkiMk; __utmz=30149280.1728978779.1.1.utmcsr=google|utmccn=(organic)|utmcmd=organic|utmctr=(not%20provided); viewed=\"1007305_27055717_10519268_36558411\"; _vwo_uuid_v2=D67C36ABC33CA10C09168D0521AD9DEA5|757429c706db3b67694302a45aec0c8c; _pk_id.100001.8cb4=e13fcef95dbf1252.1739513080.; __yadk_uid=ZLSVCuyJ41KbJVbVBgV5zd3bnFyY9HhZ; douban-fav-remind=1; dbcl2=\"264055423:qy57Xf9rC78\"; push_noty_num=0; push_doumail_num=0; __utmv=30149280.26405; ck=q-ZJ; __utmc=30149280; _pk_ses.100001.8cb4=1; __utma=30149280.212206049.1728978779.1739853169.1740022125.13; __utmt=1; __utmb=30149280.7.5.1740022125",
	},
	// Root函数使用JavaScript脚本生成初始请求数组并调用go函数AddJsReq
	Root: `
		var arr = new Array();
 		for (var i = 25; i <= 25; i+=25) {
			var obj = {
			   Url: "https://www.douban.com/group/szsh/discussion?start=" + i,
			   Priority: 1,
			   RuleName: "解析网站URL",
			   Method: "GET",
		   };
			arr.push(obj);
		};
		console.log(arr[0].Url);
		AddJsReq(arr);
			`,
	// 初始化豆瓣组爬虫任务解析规则模板，包含两个解析规则
	Rules: []spider.RuleModle{
		{
			Name: "解析网站URL",
			ParseFunc: `
			ctx.ParseJSReg("解析阳台房","(https://www.douban.com/group/topic/[0-9a-z]+/)\"[^>]*>([^<]+)</a>");
			`,
		},
		{
			Name: "解析阳台房",
			ParseFunc: `
			//console.log("parse output");
			ctx.OutputJS("<div class=\"topic-content\">[\\s\\S]*?阳台[\\s\\S]*?<div class=\"aside\">");
			`,
		},
	},
}
