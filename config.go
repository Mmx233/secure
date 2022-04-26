package secure

import (
	"github.com/gin-gonic/gin"
	"time"
)

type Config struct {
	//处理达到请求速率或黑名单ip的请求
	CallBack func(c *gin.Context)
	//请求速度限制
	RateLimit int
	//黑名单阈值
	BlackListRate int
	//黑名单封禁时间
	BlackListDuration time.Duration

	//inline data
	ipLogger   secMap
	normalList stack
	blackList  stack
}

func (c *Config) init() {
	if c.CallBack == nil {
		c.CallBack = func(c *gin.Context) {
			c.AbortWithStatus(403)
		}
	}
	c.ipLogger.init()
	c.normalList.init()
	c.blackList.init()
}
