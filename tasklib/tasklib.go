package tasklib

import (
	"github.com/dszqbsm/crawler/spider"
	"github.com/dszqbsm/crawler/tasklib/doubanbook"
	"github.com/dszqbsm/crawler/tasklib/doubangroup"
	"github.com/dszqbsm/crawler/tasklib/doubangroupjs"
)

func init() {
	spider.TaskStore.Add(doubangroup.DoubangroupTask)
	spider.TaskStore.Add(doubanbook.DoubanBookTask)
	spider.TaskStore.AddJSTask(doubangroupjs.DoubangroupJSTask)
}
