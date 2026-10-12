package config

type Role string

const (
 RoleStandalone Role = "standalone"
 RoleAPI Role = "api"
 RoleScheduler Role = "scheduler"
 RoleWorker Role = "worker"
)

func (r Role) Valid() bool {
 switch r {
 case RoleStandalone, RoleAPI, RoleScheduler, RoleWorker:
  return true
 default:
  return false
 }
}
