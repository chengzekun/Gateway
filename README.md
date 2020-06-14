项目的组织结构：

1. svrpool

    该包主要包含两个文件：
    
    一个是pool文件，该文件中保存了注册的Invoker，提供了如下的接口：
    
    - AddServer : 根据服务名，服务server的ID，将可以执行调用的Invoker对象保存到Server池中
    - RemoveInvoker : 根据服务名，服务server的ID，将相应的Invoker移出Server pool，如果对应的服务只有一个Invoker，那么会清空关于该服务的信息
    - GetInvoker : 根据服务名，服务server的ID获取到对应的Invoker
    
    需要注意的是上述的接口都是协程安全的，可以同时被多个goroutine调用，但是增删的接口最好不要频繁操作，否则会导致锁竞争激烈而使性能恶化

    另一个是scheduler文件，该文件主要是保存对应各个服务的调度器，在进行服务调用时，需要根据一定的调度策略来选出一个节点执行调用，调度器就是用来执行该功能的。
    
    但是鉴于每个服务调度器的可插拔性，我们只定义了调度器的一个接口，并且定义了一个调度器池，具体的调度器注册，移除，获取等逻辑还待完善
    
2. proxy

    proxy是该Gateway的精华所在，也是几经修改之后得到的最通用的代理，它的主要逻辑如下：
    
    - 根据传递的Query参数得到请求的服务名
        ```go
        serviceName := c.Query("service")
        ```
    - 根据获取到的服务名从调度器池中获取到一个对应的调度器（调度器的实现和注册需要由使用者来实现）
        ```go
        schedulerInstance, ok := svrpool.SchedulerPool.Load(serviceName)
        ```
    - 调用获取到的调度器的接口select得到一个Invoker（Invoker是一个接口，具体的逻辑也需要使用者来实现）
        ```go
        invoker, err := scheduler.Select()
        ```
    - 调用Invoker的Invoke方法，得到调用接口之后将结果返回给客户端
        ```go
        rsp, err := invoker.Invoke(body)
        c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "Success", "rsp": string(rsp)})
        return 
        ```
    
    从上面的逻辑可以看到，由于高度的接口化，代理的实现在之后的实现过程中基本上是不用做任何修改的，但是还有一些容错的逻辑没有完善

3. sortsvr包
    
    这是我实现的一个Demo服务：客户端向网关请求排序服务，网关通过上面的Proxy的Invoke方法将请求转发给下游的grpc server，Invoker接收到grpc的响应之后再回写给客户端
    
    当然里面包含了一些接收grpc server的注册逻辑等等，这些我也尽可能的设计的通用了，但是不太满意的是grpc server在注册时的报文设计，由于使用了非确定类型，导致很多类型断言
    
    这些内容在之后的实现中可以逐步改进，只是作为一个案例来进行展示，在之后可以注册更多的服务


