package secure

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"time"
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

	e := conf.Driver.Init(conf.RateCycle)
	if e != nil {
		return nil, e
	}

	return &Middleware{
		Handler: func(c *gin.Context) {
			rate, e := conf.Driver.AddRequest(c.ClientIP())
			if e != nil {
				fmt.Println("secure middleware store rate failed:", e)
				return
			}

			if rate >= conf.RateLimit {
				conf.HandleReachLimit(c)
				return
			}
		},
		UnBlockIP: conf.Driver.RemoveIp,
	}, nil
}
