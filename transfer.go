package secure

import (
	"container/list"
	"sync"
)

var decreasePool = &sync.Pool{
	New: func() interface{} {
		return &secDecrease{}
	},
}

type secMap struct {
	sync.RWMutex
	data map[string]*secMapCounter
}

func (a *secMap) init() {
	a.data = make(map[string]*secMapCounter, 0)
}

func (a *secMap) Load(key string) (*secMapCounter, bool) {
	a.RLock()
	defer a.RUnlock()
	v, ok := a.data[key]
	return v, ok
}

func (a *secMap) Store(key string, value *secMapCounter) {
	a.Lock()
	defer a.Unlock()
	a.data[key] = value
}

func (a *secMap) Delete(key string) {
	a.Lock()
	defer a.Unlock()
	delete(a.data, key)
}

func (a *secMap) GC() {
	a.Lock()
	defer a.Unlock()
	var data = make(map[string]*secMapCounter, len(a.data))
	for k, v := range a.data {
		data[k] = v
	}
	a.data = data
}

func (a *secMap) Clear() {
	a.Lock()
	defer a.Unlock()
	a.init()
}

type secDecrease struct {
	Ip   string
	Time int64
}

func (a *secDecrease) reset() *secDecrease {
	a.Ip = ""
	a.Time = 0
	return a
}

type secMapCounter struct {
	sync.Mutex
	Num int
}

type stack struct {
	l *list.List
	s *sync.RWMutex
}

func (a *stack) init() {
	a.l = list.New()
	a.s = new(sync.RWMutex)
}

func (a *stack) Front() *list.Element {
	a.s.RLock()
	defer a.s.RUnlock()
	return a.l.Front()
}

func (a *stack) Remove(e *list.Element) any {
	a.s.Lock()
	defer a.s.Unlock()
	return a.l.Remove(e)
}

func (a *stack) Len() int {
	a.s.RLock()
	defer a.s.RUnlock()
	return a.l.Len()
}

func (a *stack) PushBack(e interface{}) *list.Element {
	a.s.Lock()
	defer a.s.Unlock()
	return a.l.PushBack(e)
}

// nIpGC 手动从正常ip消除栈中回收某ip记录的内存占用
func (a *stack) nIpGC(ip string) {
	a.s.Lock()
	defer a.s.Unlock()
	e := a.l.Front()
	var t *list.Element
	for e != nil {
		t = e.Next()
		d := (e.Value).(*secDecrease)
		if d.Ip == ip {
			a.l.Remove(e)
			decreasePool.Put(d.reset())
		}
		e = t
	}
}
