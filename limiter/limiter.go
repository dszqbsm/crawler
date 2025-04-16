package limiter

import (
	"context"
	"sort"
	"time"

	"golang.org/x/time/rate"
)

// 限速器接口，统一了不同限速器的行为
type RateLimiter interface {
	/*
	   输入一个上下文，返回一个错误

	   遍历所有限速器，调用每个限速器的Wait方法，阻塞等待获取所有限速器的令牌，如果任何一个限速器返回错误，则返回错误，若获取到所有限速器的令牌，则返回nil
	*/
	Wait(context.Context) error // 该方法会阻塞调用者，直到可以继续执行时间，或上下文被取消
	/*
	   无输入，输出一个速率限制

	   返回第一个限速器的速率限制，因为限速器列表已经按速率限制从小到大排序，所以第一个限速器的速率限制就是所有限速器中最小的速率限制
	*/
	Limit() rate.Limit // 返回限速器的速率限制
}

/*
输入多个限速器，输出一个多限速器实例

将限速器按速率限制从小到大排序，然后返回一个多限速器示例
*/
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

/*
输入一个上下文，返回一个错误

遍历所有限速器，调用每个限速器的Wait方法，阻塞等待获取所有限速器的令牌，如果任何一个限速器返回错误，则返回错误，若获取到所有限速器的令牌，则返回nil
*/
func (l *multiLimiter) Wait(ctx context.Context) error {
	for _, l := range l.limiters {
		if err := l.Wait(ctx); err != nil {
			return err
		}
	}
	return nil
}

/*
无输入，输出一个速率限制

返回第一个限速器的速率限制，因为限速器列表已经按速率限制从小到大排序，所以第一个限速器的速率限制就是所有限速器中最小的速率限制
*/
func (l *multiLimiter) Limit() rate.Limit {
	return l.limiters[0].Limit()
}

/*
输入两个参数：eventCount：表示在指定的时间间隔内需要生成的令牌数量，duration：表示时间间隔的长度，返回一个速率限制

计算并返回在指定的时间间隔内生成指定数量的令牌的速率限制
*/
func Per(eventCount int, duration time.Duration) rate.Limit {
	return rate.Every(duration / time.Duration(eventCount))
}
