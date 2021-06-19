package secure

import (
	"container/list"
	"github.com/gin-gonic/gin"
	"sync"
	"time"
)

type secure struct {
	ipLogger      sync.Map
	n             stack
	blacklist     stack
} //防洪中间件

var Sec = secure{
	n: stack{
		l: &list.List{},
		s: &sync.RWMutex{},
	},
	blacklist: stack{
		l: &list.List{},
		s: &sync.RWMutex{},
	},
}

type secDecrease struct {
	Ip   string
	Time int64
}

type secMapCounter struct {
	NUM  int
	Lock *sync.Mutex
}

type stack struct {
	l *list.List
	s *sync.RWMutex
}

type callback interface {
	Error(c *gin.Context, code uint)
}

var CallBack callback

func (a *stack) Front() *list.Element {
	a.s.RLock()
	e := a.l.Front()
	a.s.RUnlock()
	return e
}

func (a *stack) Remove(e *list.Element) {
	a.s.Lock()
	a.l.Remove(e)
	a.s.Unlock()
}

func (a *stack) Len() int {
	a.s.RLock()
	t := a.l.Len()
	a.s.RUnlock()
	return t
}

func (a *stack) PushBack(e interface{}) {
	a.s.Lock()
	a.l.PushBack(e)
	a.s.Unlock()
}

// liveGC Map delete后无法回收占用的内存，只能remake
func (a *secure) liveGC() {
	var t sync.Map
	a.ipLogger.Range(func(key, value interface{}) bool {
		t.Store(key, value)
		return true
	})
	a.ipLogger = t
}

func (a *secure) fullGc() {
	a.ipLogger = sync.Map{}
}

// nIpGC 手动从正常ip消除栈中回收某ip记录的内存占用
func (a *secure) nIpGC(ip string) {
	e := a.n.Front()
	var t *list.Element
	for e != nil {
		t = e.Next()
		if (e.Value).(secDecrease).Ip == ip {
			a.n.Remove(e)
		}
		e = t
	}
}

func (a *secure) Init(ErrHandler callback) {
	CallBack = ErrHandler

	a.fullGc()

	{ //被动防御部分
		go func() { //normal消除执行
			for {
				if a.n.Len() == 0 {
					time.Sleep(time.Minute)
					continue
				}
				e := a.n.Front()
				t := (e.Value).(secDecrease)
				a.n.Remove(e)
				if t := t.Time - time.Now().Unix(); t > 0 {
					time.Sleep(time.Duration(t) * time.Second)
				}
				counter, ok := a.ipLogger.Load(t.Ip)
				if !ok {
					a.nIpGC(t.Ip)
					continue
				}
				counter.(*secMapCounter).Lock.Lock()
				if counter.(*secMapCounter).NUM < 0 {
					a.nIpGC(t.Ip)
					counter.(*secMapCounter).Lock.Unlock()
					continue
				}
				counter.(*secMapCounter).NUM--
				if (counter.(*secMapCounter).NUM) == 0 {
					a.ipLogger.Delete(t.Ip)
				}
				counter.(*secMapCounter).Lock.Unlock()

				if a.n.Len() == 0 {
					a.fullGc() //回收内存
				}
			}
		}()

		go func() { //黑名单消除执行
			for {
				if a.blacklist.Len() == 0 {
					a.liveGC() //回收内存
					time.Sleep(time.Hour / 2)
					continue
				}
				e := a.blacklist.Front()
				t := e.Value.(secDecrease)
				a.blacklist.Remove(e)
				if t.Time > time.Now().Unix() {
					time.Sleep(time.Duration(t.Time) * time.Second)
				}
				//解封
				a.ipLogger.Delete(t.Ip)
			}
		}()
	}
}

func (a *secure) Main() func(c *gin.Context) {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		i, ok := a.ipLogger.Load(ip)
		if !ok {
			i = &secMapCounter{
				Lock: &sync.Mutex{},
			}
			a.ipLogger.Store(ip, i)
		}
		counter := i.(*secMapCounter)
		counter.Lock.Lock()
		switch {
		case counter.NUM > 120 && counter.NUM < 300: //一分钟内最多120次访问，限制访问频次
			counter.NUM++
			a.n.PushBack(secDecrease{
				ip,
				time.Now().Unix() + 60, //60s后消除
			})
			fallthrough
		case counter.NUM < 0: //被封禁
			CallBack.Error(c, 1)
			counter.Lock.Unlock()
			return
		case counter.NUM >= 300: //每分钟超300次封禁IP
			counter.NUM = -1 //使被拦截
			secEvent := secDecrease{
				ip,
				time.Now().Unix() + 1800, //半小时后解封
			}
			a.blacklist.PushBack(secEvent)
			CallBack.Error(c, 1)
			counter.Lock.Unlock()
			return
		}
		counter.Lock.Unlock()
		c.Next()
		counter.NUM++
		a.n.PushBack(secDecrease{
			ip,
			time.Now().Unix() + 60, //60s后消除
		})
	}
}
