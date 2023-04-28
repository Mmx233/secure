package drivers

type Driver interface {
	Init() error
	RequestRate(ip string) (uint64, error)
	AddRequest(ip string) (uint64, error)
	RemoveIp(ip string) (uint64, error)
}
