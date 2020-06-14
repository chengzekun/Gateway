package sortsvr

import (
	"Gateway/stub/sortService"
	"Gateway/svrpool"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

const (
	ServiceName = "SortService"
	decay       = 0.999
)

// 表示一个提供排序服务的Server
// IP:Port是Server的唯一标识，所以一旦注册成功之后就无法更改
// Weight，CoreNum，Memory分别表示Server的权重，CPU/GPU核心数，以及内存容量，可以随时更新
// Shutdown 在Server停止想要注销服务时使用
// LastUpdate 表示Server最后一次更新的时间，需要设置一个定时任务，一旦长时间没有更新，Server就会被删除
// 其他的参数：ActivePC，AllPCCount，AvgProcessTime主要是Gateway进行统计的，可以通过这些参数计算权重，或者执行相应的负载均衡策略
// Conn 表示GateWay到提供排序服务RPC server的连接，每次进行调用时都会使用该Conn创建出一个Client去执行调用
type SortServer struct {
	IP             string           `json:"ip"`
	Port           uint16           `json:"port"`
	Weight         int32            `json:"weight"`   // 用于调度的权重
	CoreNum        int32            `json:"coreNum"`  // 核心数
	Memory         int32            `json:"memory"`   // 内存容量
	Shutdown       bool             `json:"shutdown"` // 是否停止提供服务
	LastUpdate     time.Time        // 最后一次更新的时间
	ActivePC       int64            // 活跃的调用数
	AllPCCount     int64            // 总共做了多少次
	AvgProcessTime int64            // 调用的平均时间，以微妙或者纳秒为单位
	Fail           int64            // 调用的失败次数
	Conn           *grpc.ClientConn // grpc连接，主要用于远程调用
}

type Request struct {
	Data []int32 `json:"data"`
}

func (svr *SortServer) Invoke(req []byte) ([]byte, error) {
	log.Printf("select server: %s:%d, weight: %d, active procedure call: %d, cumulative procedure call: %d\n",
		svr.IP, svr.Port, svr.Weight, svr.ActivePC, svr.AllPCCount)
	var data Request
	if err := json.Unmarshal(req, &data); err != nil {
		return nil, errors.New("unmarshal json body failed")
	}
	client := sortService.NewSortServiceClient(svr.Conn)
	ctx := context.Background()

	var sortReq sortService.SortRequest
	sortReq.Nums = data.Data
	// 修改svr的一些参数
	atomic.AddInt64(&svr.ActivePC, 1)
	atomic.AddInt64(&svr.AllPCCount, 1)
	start := time.Now()
	defer func() {
		atomic.AddInt64(&svr.ActivePC, -1)
		duration := int64(time.Now().Sub(start))
		avg := int64(float64(svr.AvgProcessTime)*decay + float64(duration)*(1-decay))
		atomic.StoreInt64(&svr.AvgProcessTime, avg)
		log.Printf("sort service cost %d milliseconds\n", duration/1000)

	}()
	rsp, err := client.Sort(ctx, &sortReq)
	if err != nil {
		log.Println("request failed, the err is", err)
		atomic.AddInt64(&svr.Fail, 1)
	}
	result, err := json.Marshal(&rsp.Nums)
	if err != nil {
		log.Println("unmarshal response body failed, the err is", err)
	}
	return result, nil
}

func RegisterSortSvr(ip string, port uint16, weight, core, memory int32) error {
	serverID := ip + ":" + strconv.Itoa(int(port))
	if invoker, err := svrpool.GetInvoker(ServiceName, serverID); err == nil { // 说明存在对应的Invoker了
		svr, _ := invoker.(*SortServer)
		svr.Shutdown = false
		return nil
	}
	svr := SortServer{IP: ip, Port: port, Weight: weight, CoreNum: core, Memory: memory, LastUpdate: time.Now()}
	opts := []grpc.DialOption{grpc.WithInsecure(), grpc.WithBlock()}
	var err error
	if svr.Conn, err = grpc.Dial(serverID, opts...); err != nil {
		log.Println("Dial server", serverID, "failed, the err is ", err)
		return err
	}
	if _, err := svrpool.AddServer(ServiceName, serverID, &svr); err != nil {
		log.Println("Add sort server into server pool failed, server ID is ", serverID, "the err is ", err)
		return err
	}
	return nil
}

func UpdateSortSvr(serverID string, updateField map[string]interface{}) error {
	invoker, err := svrpool.GetInvoker(ServiceName, serverID)
	if err != nil { // 说明存在对应的Invoker了
		return err
	}
	svr, _ := invoker.(*SortServer)
	svr.LastUpdate = time.Now()
	if len(updateField) == 0 {
		return nil
	}
	if val, exist := updateField["shutdown"]; exist {
		shutdown, ok := val.(bool)
		if !ok {
			return errors.New("wrong params, shutdown is bool type")
		}
		if shutdown {
			svrpool.RemoveInvoker(ServiceName, serverID)
			svr.Conn.Close()
			return nil
		}
	}
	for fieldName, fieldVal := range updateField {
		val, ok := fieldVal.(int32)
		if !ok {
			log.Printf("fieldVal: %v can not transform to type uint32", fieldVal)
		}
		switch fieldName {
		case "weight":
			atomic.StoreInt32(&svr.Weight, val)
		case "coreNum":
			atomic.StoreInt32(&svr.CoreNum, val)
		case "memory":
			atomic.StoreInt32(&svr.Memory, val)
		default:
			log.Println("sort server doesn't have field : ", fieldName)
		}
	}
	return nil
}

type SortServerRequest struct {
	OP      uint32                 `json:"op"`
	ServID  string                 `json:"servID"`
	SvrInfo map[string]interface{} `json:"svrInfo"`
}

const (
	Register = 1
	Update   = 2
)

var (
	once = &sync.Once{}
)

func ContactSortServer(c *gin.Context) {
	var req SortServerRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": fmt.Sprintf("%v", err)})
		return
	}
	if Register == req.OP {
		basicInfo, err := parseInfo(req.SvrInfo)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": fmt.Sprintf("format of request packet is wrong, the err is %v", err)})
			return
		}
		err = RegisterSortSvr(basicInfo.IP, basicInfo.Port, basicInfo.Weight, basicInfo.CoreNum, basicInfo.Memory)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": -2, "msg": fmt.Sprint(err)})
			return
		}
		once.Do(func() {
			svrpool.SchedulerPool.Store(ServiceName, &SortServerScheduler{})
		})
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "Success"})
		return
	}
	err := UpdateSortSvr(req.ServID, req.SvrInfo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -2, "msg": fmt.Sprint(err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "Success"})
	return
}

type BasicInfo struct {
	IP       string
	Port     uint16
	Weight   int32
	CoreNum  int32
	Memory   int32
	Shutdown bool
}

func parseInfo(fieldVals map[string]interface{}) (BasicInfo, error) {
	var basic BasicInfo
	if val, ok := fieldVals["ip"]; !ok {
		return basic, errors.New("field ip is not exist")
	} else {
		basic.IP, ok = val.(string)
		if !ok {
			return basic, errors.New("field ip is not string")
		}
	}
	if val, ok := fieldVals["port"]; !ok {
		return basic, errors.New("field port is not exist")
	} else {
		port, ok := val.(float64)
		if !ok {
			return basic, errors.New("field port is not int")
		}
		basic.Port = uint16(port)
	}
	if val, ok := fieldVals["weight"]; !ok {
		return basic, errors.New("field weight is not exist")
	} else {
		wt, ok := val.(float64)
		if !ok {
			return basic, errors.New("field weight is not int")
		}
		basic.Weight = int32(wt)
	}
	if val, ok := fieldVals["coreNum"]; !ok {
		return basic, errors.New("field coreNum is not exist")
	} else {
		core, ok := val.(float64)
		if !ok {
			return basic, errors.New("field coreNum is not int")
		}
		basic.CoreNum = int32(core)
	}
	if val, ok := fieldVals["memory"]; !ok {
		return basic, errors.New("field memory is not exist")
	} else {
		memo, ok := val.(float64)
		if !ok {
			return basic, errors.New("field memory is not int")
		}
		basic.Memory = int32(memo)
	}
	if val, ok := fieldVals["shutdown"]; !ok {
		return basic, errors.New("field shutdown is not exist")
	} else {
		shutdown, ok := val.(bool)
		if !ok {
			return basic, errors.New("field shutdown is not bool")
		}
		basic.Shutdown = shutdown
	}
	return basic, nil
}

type SortServerScheduler struct {
}

func (scheduler *SortServerScheduler) Select() (svrpool.Invoker, error) {
	serverInstance, ok := svrpool.ServerPool.Load(ServiceName)
	if !ok {
		return nil, errors.New("service doesn't exist")
	}
	svrs, _ := serverInstance.(svrpool.Servers)
	svrs.RWLock.RLock()
	defer svrs.RWLock.RUnlock()
	var weightSum int32
	for _, svr := range *svrs.SvrSlice {
		sortSvr, _ := svr.(*SortServer)
		weightSum += sortSvr.Weight
	}
	time.Sleep(10 * time.Millisecond)
	rand.Seed(time.Now().Unix())
	randWt := rand.Int31n(weightSum)
	for _, svr := range *svrs.SvrSlice {
		sortSvr, _ := svr.(*SortServer)
		randWt -= sortSvr.Weight
		if randWt < 0 {
			// log.Printf("Choose the server : %+v", *sortSvr)
			return sortSvr, nil
		}
	}
	return nil, errors.New("can not get one instance")
}
