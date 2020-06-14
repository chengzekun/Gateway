package proxy

import (
	"Gateway/svrpool"
	"io/ioutil"
	"net/http"

	"github.com/gin-gonic/gin"
)

func Proxy(c *gin.Context) {
	serviceName := c.Query("service")
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "can not read request body", "rsp": nil})
		return
	}
	schedulerInstance, ok := svrpool.SchedulerPool.Load(serviceName)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "can not get scheduler instance", "rsp": nil})
		return
	}
	var scheduler svrpool.Scheduler
	scheduler, ok = schedulerInstance.(svrpool.Scheduler)
	invoker, err := scheduler.Select()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "can not get one instance", "rsp": nil})
		return
	}
	rsp, err := invoker.Invoke(body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "msg": "request for service failed", "rsp": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "Success", "rsp": string(rsp)})
	return
}
