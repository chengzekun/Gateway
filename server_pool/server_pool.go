package server_pool

import (
	"errors"
	"net/http"
	"sync"
)

type Invoker interface {
	Invoke(r http.Request) ([]byte, error)
}

type Servers struct {
	rwLock   *sync.RWMutex      // 添加和移除server时使用
	SvrMap   map[string]Invoker // Map 和 Slice 中保存的其实是同一份Server，并且保存的都只是指针，指向相同的Server对象
	SvrSlice []Invoker
	SvrCount int
}

var (
	ServerPool  = &sync.Map{}   // serviceName -> Servers 的映射
	svrPoolLock = &sync.Mutex{} // 在向ServerPool中添加
)

const (
	AddInvokerErrorRepeatAdd           = -1 // 添加Invoker时的错误码，表示重复添加
	RemoveInvokerErrorServiceNotExists = -1 // 移除Invoker时的错误码，表示service不存在
	RemoveInvokerErrorServerNotExists  = -2 // 移除Invoker时的状态码，表示Invoker不存在
)

func AddServer(serviceName, serverID string, invoker Invoker) (int, error) {
	svrs, ok := ServerPool.Load(serviceName)
	if ok {
		serversInstance, _ := svrs.(Servers)
		serversInstance.rwLock.Lock()
		if _, exist := serversInstance.SvrMap[serverID]; exist {
			serversInstance.rwLock.Unlock()
			return AddInvokerErrorRepeatAdd, errors.New("repeated add")
		}
		serversInstance.SvrMap[serverID] = invoker
		serversInstance.SvrSlice = append(serversInstance.SvrSlice, invoker)
		serversInstance.rwLock.Unlock()
		return 0, nil
	}
	svrPoolLock.Lock()
	defer svrPoolLock.Unlock()
	// 再次检查是因为担心在竞争锁时已经被其他的goroutine捷足先登，这样可能会导致被覆盖
	if svrs, ok = ServerPool.Load(serviceName); ok {
		return AddServer(serviceName, serverID, invoker)
	}
	svrs = Servers{rwLock: &sync.RWMutex{}, SvrMap: map[string]Invoker{serverID: invoker}, SvrSlice: []Invoker{invoker}}
	ServerPool.Store(serviceName, svrs)
	return 0, nil
}

func RemoveInvoker(serviceName, serverID string) (int, error) {
	svrs, ok := ServerPool.Load(serviceName)
	if !ok {
		return RemoveInvokerErrorServiceNotExists, errors.New("service doesn't exist")
	}
	serversInstance, _ := svrs.(Servers)
	serversInstance.rwLock.Lock()
	defer serversInstance.rwLock.Unlock()
	// 删除invoker 的逻辑
	invoker, exist := serversInstance.SvrMap[serverID]
	if !exist {
		return RemoveInvokerErrorServerNotExists, errors.New("server doesn't exist")
	}
	delete(serversInstance.SvrMap, serverID)
	index := 0
	for i, instance := range serversInstance.SvrSlice {
		if instance == invoker {
			index = i
			break
		}
	}
	serversInstance.SvrSlice = append(serversInstance.SvrSlice[0:index], serversInstance.SvrSlice[index+1:]...)
	if len(serversInstance.SvrMap) > 0 {
		return 0, nil
	}
	svrPoolLock.Lock()
	defer svrPoolLock.Unlock()
	if len(serversInstance.SvrMap) > 0 {
		return 0, nil
	}
	ServerPool.Delete(serviceName)
	return 0, nil
}
