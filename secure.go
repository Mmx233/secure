package secure

import (
	"container/list"
	"github.com/gin-gonic/gin"
	"sync"
	"time"
)

var ipLogger sync.Map
var n = stack{
	l: &list.List{},
	s: &sync.RWMutex{},
}
var blacklist = stack{
	l: &list.List{},
	s: &sync.RWMutex{},
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

var callBack func(c *gin.Context)

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
func liveGC() {
	var t sync.Map
	ipLogger.Range(func(key, value interface{}) bool {
		t.Store(key, value)
		return true
	})
	ipLogger = t
}

func fullGc() {
	ipLogger = sync.Map{}
}

// nIpGC 手动从正常ip消除栈中回收某ip记录的内存占用
func nIpGC(ip string) {
	e := n.Front()
	var t *list.Element
	for e != nil {
		t = e.Next()
		if (e.Value).(secDecrease).Ip == ip {
			n.Remove(e)
		}
		e = t
	}
}

func Init(ErrHandler func(c *gin.Context)) {
	callBack = ErrHandler

	fullGc()

	{ //被动防御部分
		go func() { //normal消除执行
			for {
				if n.Len() == 0 {
					time.Sleep(time.Minute)
					continue
				}
				e := n.Front()
				t := (e.Value).(secDecrease)
				n.Remove(e)
				if t := t.Time - time.Now().Unix(); t > 0 {
					time.Sleep(time.Duration(t) * time.Second)
				}
				counter, ok := ipLogger.Load(t.Ip)
				if !ok {
					nIpGC(t.Ip)
					continue
				}
				counter.(*secMapCounter).Lock.Lock()
				if counter.(*secMapCounter).NUM < 0 {
					nIpGC(t.Ip)
					counter.(*secMapCounter).Lock.Unlock()
					continue
				}
				counter.(*secMapCounter).NUM--
				if (counter.(*secMapCounter).NUM) == 0 {
					ipLogger.Delete(t.Ip)
				}
				counter.(*secMapCounter).Lock.Unlock()

				if n.Len() == 0 {
					fullGc() //回收内存
				}
			}
		}()

		go func() { //黑名单消除执行
			for {
				if blacklist.Len() == 0 {
					liveGC() //回收内存
					time.Sleep(time.Hour / 2)
					continue
				}
				e := blacklist.Front()
				t := e.Value.(secDecrease)
				blacklist.Remove(e)
				if t.Time > time.Now().Unix() {
					time.Sleep(time.Duration(t.Time) * time.Second)
				}
				//解封
				ipLogger.Delete(t.Ip)
			}
		}()
	}
}

func Main() func(c *gin.Context) {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		i, ok := ipLogger.Load(ip)
		if !ok {
			i = &secMapCounter{
				Lock: &sync.Mutex{},
			}
			ipLogger.Store(ip, i)
		}
		counter := i.(*secMapCounter)
		counter.Lock.Lock()
		switch {
		case counter.NUM > 120 && counter.NUM < 300: //一分钟内最多120次访问，限制访问频次
			counter.NUM++
			n.PushBack(secDecrease{
				ip,
				time.Now().Unix() + 60, //60s后消除
			})
			fallthrough
		case counter.NUM < 0: //被封禁
			callBack(c)
			counter.Lock.Unlock()
			return
		case counter.NUM >= 300: //每分钟超300次封禁IP
			counter.NUM = -1 //使被拦截
			secEvent := secDecrease{
				ip,
				time.Now().Unix() + 1800, //半小时后解封
			}
			blacklist.PushBack(secEvent)
			callBack(c)
			counter.Lock.Unlock()
			return
		}
		counter.Lock.Unlock()
		c.Next()
		counter.NUM++
		n.PushBack(secDecrease{
			ip,
			time.Now().Unix() + 60, //60s后消除
		})
	}
}
