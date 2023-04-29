package drivers

import "time"

type Driver interface {
	Init(rateCycle time.Duration) error
	AddRequest(ip string) (uint64, error)
	RemoveIp(ip string) (uint64, error)
}
