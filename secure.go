package secure

import (
	"github.com/gin-gonic/gin"
	"time"
)

func New(conf *Config) gin.HandlerFunc {
	conf.init()
	go func() { //normal消除执行
		for {
			if conf.normalList.Len() == 0 {
				time.Sleep(time.Minute)
				continue
			}
			e := conf.normalList.Front()
			t := (e.Value).(*secDecrease)
			conf.normalList.Remove(e)
			if t := t.Time - time.Now().Unix(); t > 0 {
				time.Sleep(time.Duration(t) * time.Second)
			}
			counter, ok := conf.ipLogger.Load(t.Ip)
			if !ok {
				conf.normalList.nIpGC(t.Ip)
				continue
			}
			counter.Lock()
			if counter.Num < 0 {
				conf.normalList.nIpGC(t.Ip)
				counter.Unlock()
				continue
			}
			counter.Num--
			if (counter.Num) == 0 {
				conf.ipLogger.Delete(t.Ip)
			}
			counter.Unlock()

			//pool回收
			decreasePool.Put(t.reset())

			if conf.normalList.Len() == 0 {
				conf.ipLogger.GC() //回收内存
			}
		}
	}()

	go func() { //黑名单消除执行
		for {
			if conf.blackList.Len() == 0 {
				conf.ipLogger.GC() //闲时回收内存
				time.Sleep(time.Hour / 2)
				continue
			}
			e := conf.blackList.Front()
			t := e.Value.(*secDecrease)
			conf.blackList.Remove(e)
			if t.Time > time.Now().Unix() {
				time.Sleep(time.Duration(t.Time) * time.Second)
			}
			//解封
			conf.ipLogger.Delete(t.Ip)

			//pool回收
			decreasePool.Put(t.reset())
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		counter, ok := conf.ipLogger.Load(ip)
		if !ok {
			counter = &secMapCounter{}
			conf.ipLogger.Store(ip, counter)
			return
		}
		counter.Lock()
		defer counter.Unlock()
		switch {
		case counter.Num < 0: //被封禁
			conf.CallBack(c)
			return
		case counter.Num < conf.BlackListRate: //限制访问频次
			counter.Num++
			d := decreasePool.Get().(*secDecrease)
			d.Ip = ip
			d.Time = time.Now().Unix() + 60
			conf.normalList.PushBack(d)
			if counter.Num > conf.RateLimit {
				conf.CallBack(c)
			}
			return
		default: //封禁IP
			counter.Num = -1 //使被拦截
			secEvent := &secDecrease{
				ip,
				time.Now().Add(conf.BlackListDuration).Unix(), //解封
			}
			conf.blackList.PushBack(secEvent)
			conf.CallBack(c)
			return
		}
	}
}
