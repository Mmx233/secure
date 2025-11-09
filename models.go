package secure

import (
	"time"

	"github.com/Mmx233/secure/v2/drivers"
	"github.com/gin-gonic/gin"
)

type Config struct {
	Driver           drivers.Driver
	HandleReachLimit func(*gin.Context)

	// 速率统计周期，默认一分钟
	RateCycle time.Duration
	// 速率限制
	RateLimit       uint64
	UnderAttackMode bool
}
