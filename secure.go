package secure

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

type Middleware struct {
	Handler   func(*gin.Context)
	UnBlockIP func(ip string) (uint64, error)
}

func New(conf *Config) (*Middleware, error) {
	if conf.RateCycle == 0 {
		conf.RateCycle = time.Minute
	}
	if conf.RateLimit == 0 {
		conf.RateLimit = 120
	}
	if conf.HandleReachLimit == nil {
		conf.HandleReachLimit = func(c *gin.Context) {
			c.AbortWithStatus(403)
		}
	}

	if err := conf.Driver.Init(conf.RateCycle); err != nil {
		return nil, err
	}

	var middleware = Middleware{
		UnBlockIP: conf.Driver.RemoveIp,
	}
	if conf.UnderAttackMode {
		middleware.Handler = func(c *gin.Context) {
			rate, err := conf.Driver.AddRequest(c.ClientIP())
			if err != nil {
				fmt.Println("secure middleware store rate failed:", err)
				return
			}

			if rate >= conf.RateLimit {
				conf.HandleReachLimit(c)
				return
			}
		}
	} else {
		middleware.Handler = func(c *gin.Context) {
			ip := c.ClientIP()
			rate, err := conf.Driver.RequestRate(ip)
			if err != nil {
				fmt.Println("secure middleware read rate failed:", err)
				return
			}

			if rate >= conf.RateLimit {
				conf.HandleReachLimit(c)
				return
			}

			_, err = conf.Driver.AddRequest(ip)
			if err != nil {
				fmt.Println("secure middleware store rate failed:", err)
			}
		}
	}
	return &middleware, nil
}
