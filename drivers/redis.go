package drivers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v8"
	"strconv"
	"time"
)

const redisDefaultKey = "secure"

type RedisDriver struct {
	Key    string // redis 存储键名
	Client *redis.Client

	cycle time.Duration

	keyWaitList string
}

func (a *RedisDriver) ipKey(ip string) string {
	return a.Key + "-" + ip
}

func (a *RedisDriver) Init(rateCycle time.Duration) error {
	a.cycle = rateCycle
	if a.Key == "" {
		a.Key = redisDefaultKey
	}
	a.keyWaitList = a.Key + "-wait-list"
	go a.QueueWorker()
	return nil
}

func (a *RedisDriver) RequestRate(ip string) (uint64, error) {
	rateStr, e := a.Client.Get(context.Background(), a.ipKey(ip)).Result()
	if e == redis.Nil {
		e = nil
	}
	rate, _ := strconv.ParseUint(rateStr, 10, 64)
	return rate, e
}

func (a *RedisDriver) AddRequest(ip string) (uint64, error) {
	ip = a.ipKey(ip)
	el, e := json.Marshal(&IpQueueEl{
		Key:      ip,
		CreateAt: time.Now(),
	})
	if e != nil {
		return 0, e
	}
	e = a.Client.LPush(context.Background(), a.keyWaitList, string(el)).Err()
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
	rate, e := a.Client.Del(context.Background(), a.ipKey(ip)).Uint64()
	if e == redis.Nil {
		e = nil
	}
	return rate, e
}

func (a *RedisDriver) QueueWorker() {
	for {
		el, e := a.Client.RPop(context.Background(), a.keyWaitList).Bytes()
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

		rate, e := a.Client.Decr(context.Background(), ipInfo.Key).Uint64()
		if rate <= 0 && e == nil {
			_, _ = a.RemoveIp(ipInfo.Key)
		}
	}
}
