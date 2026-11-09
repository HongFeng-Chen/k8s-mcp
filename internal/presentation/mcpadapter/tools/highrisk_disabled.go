// 高危工具组占位说明（v1 不实现）。
//
// 设计原则：exec / delete / create 属于高危操作，默认禁用，
// 未来开放必须同时满足：全局开关 + 人工审批流 + 领域层 RiskHigh 拦截。
//
// 为什么 exec_pod 第一版绝不实现：
//   - 进入容器后可以读写挂载的 ServiceAccount Token（横向移动）；
//   - hostPath / privileged 配置下可直接逃逸到宿主机；
//   - LLM 幻觉执行的破坏面不可控，且事故难以追溯。
//
// 领域层已预留对应等级与错误：
//   - service.OpExec / OpDelete / OpCreate -> RiskHigh
//   - domainerrors.ErrHighRiskNeedApproval
//
// 删除/创建类高危操作请走 GitOps 或人工 kubectl（经正式审批）。
package tools
