# SetNestedAttribute 非 identity readOnly 字段变化对照用例

## 用例性质

这是一个**未复现问题的阴性对照用例**，不是 `null -> unknown` 正例。

测试对象是：

```text
Volcengine::VPC::SecurityGroup.IngressPermissions
Terraform 类型：SetNestedAttribute
```

## 验证内容

用例先创建包含两条 ingress rule 的 SecurityGroup，然后通过 Cloud Control：

1. 临时修改第一条规则的 `Description`；
2. 再把 `Description` 恢复为 Terraform 配置中的原值；
3. 让服务端重新生成嵌套的 `CreationTime` 和 `UpdateTime`；
4. 在业务配置不变、只有 readOnly 时间字段变化的情况下重新执行
   `terraform plan`。

## 实际结果

最终 plan 结果为：

```text
No changes. Your infrastructure matches the configuration.
AFTER_RESTORE_PLAN_EXIT_CODE=0
```

因此，该用例证明：

> 在已测的 `IngressPermissions` case 中，非 identity readOnly 时间字段变化没有
> 造成无意义 plan。

## 证据边界

该结果只能说明当前 SecurityGroup case 未复现问题，不能推广为所有
`SetNestedAttribute` 都不会受非 identity 字段变化影响。

本用例没有制造或复现 computed/readback 字段的 `null -> unknown`，因此不应作为
`plan_null_unknown` 问题的正向复现证据。

## 证据文件

测试配置：

```text
main.tf
```

执行日志：

```text
logs/plan-non-identity-apply.log
logs/plan-non-identity-baseline.log
logs/plan-non-identity-change-and-restore.log
logs/plan-non-identity-after-restore.log
logs/plan-non-identity-destroy.log
logs/plan-non-identity-state-list-after-destroy.log
```

原始完整测试记录：

```text
../../e2e_reports_backup/e2e_plan_non_identity_result.md
```
