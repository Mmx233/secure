package drivers

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// DefaultDriver 基于内存的速率统计方法
type DefaultDriver struct {
	data     map[string]*atomic.Uint64
	dataLock sync.Mutex

	period time.Duration

	queue FreeLockQueue
}

func (a *DefaultDriver) Init(rateCycle time.Duration) error {
	a.period = rateCycle
	a.data = make(map[string]*atomic.Uint64)
	a.queue.Init()
	go a.QueueWorker()
	return nil
}

func (a *DefaultDriver) AddRequest(ip string) (uint64, error) {
	num, ok := a.data[ip]
	if !ok {
		a.dataLock.Lock()
		num, ok = a.data[ip]
		if !ok {
			num = &atomic.Uint64{}
			a.data[ip] = num
		}
		a.dataLock.Unlock()
	}
	a.queue.Enqueue(&IpQueueEl{
		IP:       ip,
		CreateAt: time.Now(),
	})
	return num.Add(uint64(1)), nil
}

func (a *DefaultDriver) RequestRate(ip string) (uint64, error) {
	num, ok := a.data[ip]
	if !ok {
		return 0, nil
	}
	return num.Load(), nil
}

func (a *DefaultDriver) RemoveIp(ip string) (uint64, error) {
	num, ok := a.data[ip]
	if ok {
		a.dataLock.Lock()
		delete(a.data, ip)
		if len(a.data) == 0 {
			a.data = make(map[string]*atomic.Uint64)
		}
		a.dataLock.Unlock()
	}
	return num.Load(), nil
}

func (a *DefaultDriver) QueueWorker() {
	for {
		el := a.queue.Dequeue()
		if el == nil {
			time.Sleep(a.period)
			continue
		}
		ipInfo := el.(*IpQueueEl)
		subTime := a.period - time.Now().Sub(ipInfo.CreateAt)
		if subTime > 0 {
			time.Sleep(subTime)
		}

		num, ok := a.data[ipInfo.IP]
		if ok && num.Add(^uint64(0)) <= 0 {
			a.dataLock.Lock()
			num, ok = a.data[ipInfo.IP]
			if ok && num.Load() <= 0 {
				delete(a.data, ipInfo.IP)
			}
			a.dataLock.Unlock()
		}
	}
}

type QueueElement struct {
	Value interface{}
	Next  unsafe.Pointer
}

// FreeLockQueue 从尾部加入，从头部添加
type FreeLockQueue struct {
	head unsafe.Pointer
	tail unsafe.Pointer
}

func (a *FreeLockQueue) Init() {
	a.head = unsafe.Pointer(&QueueElement{})
	a.tail = a.head
}

func (a *FreeLockQueue) Enqueue(value interface{}) {
	n := unsafe.Pointer(&QueueElement{
		Value: value,
	})
	for {
		tail := (*QueueElement)(a.tail)
		next := tail.Next
		if tail == (*QueueElement)(a.tail) {
			if next == nil {
				if atomic.CompareAndSwapPointer(&tail.Next, next, n) {
					atomic.CompareAndSwapPointer(&a.tail, unsafe.Pointer(tail), n)
				}
			} else { // 队列尾部异常，未指向正确元素
				atomic.CompareAndSwapPointer(&a.tail, unsafe.Pointer(tail), next)
			}
		}
	}
}

func (a *FreeLockQueue) Dequeue() interface{} {
	for {
		head := (*QueueElement)(a.head)
		tail := (*QueueElement)(a.tail)
		next := head.Next
		if head == (*QueueElement)(a.head) {
			if head == tail {
				if next == nil { // 队列为空
					return nil
				}
				// 队尾异常
				atomic.CompareAndSwapPointer(&a.head, unsafe.Pointer(head), next)
			} else {
				if atomic.CompareAndSwapPointer(&a.head, unsafe.Pointer(head), next) {
					return head.Value
				}
			}
		}
	}
}
