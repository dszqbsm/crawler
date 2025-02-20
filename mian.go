package main

import (
	"time"

	"github.com/dszqbsm/crawler/collect"
	"github.com/dszqbsm/crawler/engine"
	"github.com/dszqbsm/crawler/log"
	"go.uber.org/zap/zapcore"
)

func main() {
	// log
	plugin := log.NewStdoutPlugin(zapcore.InfoLevel)
	logger := log.NewLogger(plugin)
	logger.Info("log init end")

	var f collect.Fetcher = &collect.BrowserFetch{
		Timeout: 3000 * time.Millisecond,
		Logger:  logger,
		Proxy:   nil,
	}
	seeds := make([]*collect.Task, 0, 1000)

	seeds = append(seeds, &collect.Task{
		Property: collect.Property{
			Name: "js_find_douban_sun_room",
		},
		Fetcher: f,
	})

	s := engine.NewEngine(
		engine.WithFetcher(f),
		engine.WithLogger(logger),
		engine.WithWorkCount(5),
		engine.WithSeeds(seeds),
		engine.WithScheduler(engine.NewSchedule()),
	)

	s.Run()
}
