# secure

## 使用

```shell
~$ go get github.com/Mmx233/secure/v2
```

基于内存记录访问信息

```go
package middlewares

import (
	"github.com/Mmx233/secure/v2"
	"github.com/Mmx233/secure/v2/drivers"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func Secure() gin.HandlerFunc {
	middleware, e := secure.New(&secure.Config{
		Driver: &drivers.DefaultDriver{},
		HandleReachLimit: func(c *gin.Context) { // 处理到达访问速率的请求
			c.AbortWithStatus(403)
		},
		RateCycle:       0, // 计数周期，默认一分钟
		RateLimit:       0, // 最大访问速率，默认 120
		UnderAttackMode: false, // 已拦截后继续计数
	})
	if e != nil {
		log.Fatalln(e)
	}
	return middleware.Handler
}

```

使用 redis 记录访问信息

```go
package middlewares

import (
	"github.com/Mmx233/secure/v2"
	"github.com/Mmx233/secure/v2/drivers"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
	"time"
)

func Secure() gin.HandlerFunc {
	middleware, e := secure.New(&secure.Config{
		Driver: &drivers.RedisDriver{
			Client: redis.NewClient(&redis.Options{
				Addr:       "redisAddr:6379",
				Password:   "password",
				DB:         0,
				MaxConnAge: time.Hour * 5,
			}),
		},
		HandleReachLimit: func(c *gin.Context) {
			c.AbortWithStatus(403)
		},
	})
	if e != nil {
		log.Fatalln(e)
	}
	return middleware.Handler
}

```