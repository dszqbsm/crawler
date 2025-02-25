package limiter

import (
	"context"
	"sort"
	"time"

	"golang.org/x/time/rate"
)

// 限速器接口，统一了不同限速器的行为
type RateLimiter interface {
	Wait(context.Context) error // 该方法会阻塞调用者，知道可以继续执行时间，或上下文被取消
	Limit() rate.Limit          // 返回限速器的速率限制
}

// 将多个限速器按速率限制从小到大排序，然后返回一个多限速器实例
func Multi(limiters ...RateLimiter) *multiLimiter {
	// 定义一个比较函数，byLimit用于比较两个限速器的速率限制
	byLimit := func(i, j int) bool {
		return limiters[i].Limit() < limiters[j].Limit()
	}
	// 使用sort.Slice函数对传入的限速器列表按速率限制从小到大排序
	sort.Slice(limiters, byLimit)
	// 返回多限速器示例，包含排序后的限速器列表
	return &multiLimiter{limiters: limiters}
}

// 多限速器结构体，用于实现多个限速器的组合
type multiLimiter struct {
	limiters []RateLimiter
}

// 用于等待可用的令牌，若没有可用的令牌，当前协程会陷入阻塞状态，直到有可用的令牌或上下文被取消
func (l *multiLimiter) Wait(ctx context.Context) error {
	// 遍历所有限速器，调用每个限速器的Wait方法，如果任何一个限速器返回错误，则返回错误，若所有限速器都允许执行，则返回nil
	for _, l := range l.limiters {
		if err := l.Wait(ctx); err != nil {
			return err
		}
	}
	return nil
}

// 返回第一个限速器的速率限制
func (l *multiLimiter) Limit() rate.Limit {
	return l.limiters[0].Limit()
}

// Every函数用于指定两个令牌之间的时间间隔
func Per(eventCount int, duration time.Duration) rate.Limit {
	return rate.Every(duration / time.Duration(eventCount))
}
