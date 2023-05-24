package drivers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v8"
	"time"
)

const redisWaitListKey = "ip-wait-list"

type RedisDriver struct {
	Key    string // redis 存储键名
	Client *redis.Client

	cycle time.Duration
}

func (a *RedisDriver) Init(rateCycle time.Duration) error {
	a.cycle = rateCycle
	if a.Key == "" {
		a.Key = redisWaitListKey
	}
	go a.QueueWorker()
	return nil
}

func (a *RedisDriver) RequestRate(ip string) (uint64, error) {
	rate, e := a.Client.Get(context.Background(), ip).Uint64()
	if e == redis.Nil {
		e = nil
	}
	return rate, e
}

func (a *RedisDriver) AddRequest(ip string) (uint64, error) {
	el, e := json.Marshal(&IpQueueEl{
		IP:       ip,
		CreateAt: time.Now(),
	})
	if e != nil {
		return 0, e
	}
	e = a.Client.LPush(context.Background(), redisWaitListKey, string(el)).Err()
	if e != nil {
		return 0, e
	}
	rate, e := a.Client.Incr(context.Background(), ip).Uint64()
	if e != nil {
		return 0, e
	}
	return rate, a.Client.Expire(context.Background(), ip, a.cycle).Err()
}

func (a *RedisDriver) RemoveIp(ip string) (uint64, error) {
	rate, e := a.Client.Del(context.Background(), ip).Uint64()
	if e == redis.Nil {
		e = nil
	}
	return rate, e
}

func (a *RedisDriver) QueueWorker() {
	for {
		el, e := a.Client.RPop(context.Background(), redisWaitListKey).Bytes()
		if e != nil {
			time.Sleep(a.cycle)
			continue
		}
		var ipInfo IpQueueEl
		e = json.Unmarshal(el, &ipInfo)
		if e != nil {
			fmt.Println("unexpected error: secure ip wait list element unmarshal failed:", e)
			continue
		}

		subTime := a.cycle - time.Now().Sub(ipInfo.CreateAt)
		if subTime > 0 {
			time.Sleep(subTime)
		}

		rate, e := a.Client.Decr(context.Background(), ipInfo.IP).Uint64()
		if rate <= 0 && e == nil {
			_, _ = a.RemoveIp(ipInfo.IP)
		}
	}
}
