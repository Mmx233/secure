package drivers

import "time"

type Driver interface {
	Init(rateCycle time.Duration) error
	RequestRate(ip string) (uint64, error)
	AddRequest(ip string) (uint64, error)
	RemoveIp(ip string) (uint64, error)
}
