package drivers

import (
	"sync"
	"sync/atomic"
	"time"
)

// DefaultDriver 基于内存的速率统计方法
type DefaultDriver struct {
	data  sync.Map
	queue FreeLockQueue

	period time.Duration
}

func (a *DefaultDriver) Init(rateCycle time.Duration) error {
	a.period = rateCycle
	a.queue.Init()
	go a.QueueWorker()
	return nil
}

func (a *DefaultDriver) AddRequest(ip string) (uint64, error) {
	numInterface, _ := a.data.LoadOrStore(ip, &atomic.Uint64{})
	num := numInterface.(*atomic.Uint64)
	a.queue.Enqueue(&IpQueueElement{
		Key:      ip,
		CreateAt: time.Now(),
	})
	return num.Add(1), nil
}

func (a *DefaultDriver) RequestRate(ip string) (uint64, error) {
	num, ok := a.data.Load(ip)
	if !ok {
		return 0, nil
	}
	return num.(*atomic.Uint64).Load(), nil
}

func (a *DefaultDriver) RemoveIp(ip string) (uint64, error) {
	num, ok := a.data.LoadAndDelete(ip)
	if ok {
		return num.(*atomic.Uint64).Load(), nil
	}
	return 0, nil
}

func (a *DefaultDriver) QueueWorker() {
	for {
		el := a.queue.Dequeue()
		if el == nil {
			time.Sleep(a.period)
			continue
		}
		subTime := a.period - time.Now().Sub(el.CreateAt)
		if subTime > 0 {
			time.Sleep(subTime)
		}

		num, ok := a.data.Load(el.Key)
		if ok && num.(*atomic.Uint64).Add(^uint64(0)) <= 0 {
			a.data.Delete(el.Key)
		}
	}
}

type QueueElement struct {
	Value atomic.Pointer[IpQueueElement]
	Next  atomic.Pointer[QueueElement]
}

// FreeLockQueue 从尾部加入，从头部添加
type FreeLockQueue struct {
	head atomic.Pointer[QueueElement]
	tail atomic.Pointer[QueueElement]
}

func (a *FreeLockQueue) Init() {
	a.head.Store(&QueueElement{})
	a.tail.Store(a.head.Load())
}

func (a *FreeLockQueue) Enqueue(value *IpQueueElement) {
	n := &QueueElement{}
	for {
		tail := a.tail.Load()
		if tail.Value.CompareAndSwap(nil, value) {
			if !tail.Next.CompareAndSwap(nil, n) || !a.tail.CompareAndSwap(tail, n) {
				panic("secure middleware: invalid queue element")
			}
			break
		}
	}
}

func (a *FreeLockQueue) Dequeue() *IpQueueElement {
	for {
		head := a.head.Load()
		next := head.Next.Load()
		if head == a.tail.Load() {
			if next == nil {
				return nil
			} else {
				// adding first element
			}
		}
		if a.head.CompareAndSwap(head, next) {
			return head.Value.Load()
		}
	}
}
