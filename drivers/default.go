package drivers

import (
	"os"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// DefaultDriver 基于内存的速率统计方法
type DefaultDriver struct {
	data sync.Map

	period time.Duration

	queue FreeLockQueue
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
	a.queue.Enqueue(unsafe.Pointer(&IpQueueEl{
		Key:      ip,
		CreateAt: time.Now(),
	}))
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
		ipInfo := (*IpQueueEl)(el)
		subTime := a.period - time.Now().Sub(ipInfo.CreateAt)
		if subTime > 0 {
			time.Sleep(subTime)
		}

		num, ok := a.data.Load(ipInfo.Key)
		if ok && num.(*atomic.Uint64).Add(^uint64(0)) <= 0 {
			a.data.Delete(ipInfo.Key)
		}
	}
}

type QueueElement struct {
	Value unsafe.Pointer
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

func (a *FreeLockQueue) Enqueue(value unsafe.Pointer) {
	n := unsafe.Pointer(&QueueElement{})
	for {
		tail := (*QueueElement)(a.tail)
		next := tail.Next
		if atomic.CompareAndSwapPointer(&tail.Value, nil, value) {
			if !atomic.CompareAndSwapPointer(&tail.Next, next, n) || !atomic.CompareAndSwapPointer(&a.tail, unsafe.Pointer(tail), n) {
				// unexpected enqueue movement
				os.Exit(1)
			} else {
				return
			}
		}
	}
}

func (a *FreeLockQueue) Dequeue() unsafe.Pointer {
	for {
		head := (*QueueElement)(a.head)
		next := head.Next
		tail := (*QueueElement)(a.tail)
		if head == (*QueueElement)(a.head) {
			if head == tail {
				if next == nil { // 队列为空
					return nil
				}
				// 队列正在添加第一个元素
			}
			if atomic.CompareAndSwapPointer(&a.head, unsafe.Pointer(head), next) {
				return head.Value
			}
		}
	}
}
