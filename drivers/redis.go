package drivers

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
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
	rate, err := a.Client.Get(context.Background(), a.ipKey(ip)).Uint64()
	if err != nil {
		var numError *strconv.NumError
		if errors.As(err, &numError) || errors.Is(err, redis.Nil) {
			err = nil
		}
	}
	return rate, err
}

func (a *RedisDriver) AddRequest(ip string) (uint64, error) {
	ip = a.ipKey(ip)
	el, err := json.Marshal(&IpQueueElement{
		Key:      ip,
		CreateAt: time.Now(),
	})
	if err != nil {
		return 0, err
	}
	err = a.Client.LPush(context.Background(), a.keyWaitList, string(el)).Err()
	if err != nil {
		return 0, err
	}
	rate, err := a.Client.Incr(context.Background(), ip).Uint64()
	if err != nil {
		return 0, err
	}
	return rate, a.Client.Expire(context.Background(), ip, a.cycle).Err()
}

func (a *RedisDriver) RemoveIp(ip string) (uint64, error) {
	rate, err := a.Client.Del(context.Background(), a.ipKey(ip)).Uint64()
	if errors.Is(err, redis.Nil) {
		err = nil
	}
	return rate, err
}

func (a *RedisDriver) QueueWorker() {
	for {
		el, err := a.Client.RPop(context.Background(), a.keyWaitList).Bytes()
		if err != nil {
			time.Sleep(a.cycle)
			continue
		}
		var ipInfo IpQueueElement
		err = json.Unmarshal(el, &ipInfo)
		if err != nil {
			panic("secure middleware: list element unmarshal failed: " + err.Error())
		}

		subTime := a.cycle - time.Now().Sub(ipInfo.CreateAt)
		if subTime > 0 {
			time.Sleep(subTime)
		}

		rate, err := a.Client.Decr(context.Background(), ipInfo.Key).Uint64()
		if rate <= 0 && err == nil {
			_, _ = a.RemoveIp(ipInfo.Key)
		}
	}
}
