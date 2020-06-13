package server_pool

type Scheduler interface {
	Select(serviceName string) (Invoker, error)
}
