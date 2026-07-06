# SetNestedAttribute 稳定第二次 Plan `null -> unknown` E2E 结果

## 结论

使用 `origin/main` 最新提交 `fe912e8600d5c09f239d54efe4c9bd1db10d1cb5`
构建 Provider 后，已真实复现：

1. 首次 `terraform apply` 成功；
2. 不修改 `main.tf`；
3. 第二次 `terraform plan -detailed-exitcode` 返回 `2`；
4. `volcenginecc_vpc_eni.private_ip_sets` 中非 identity 字段
   `associated_elastic_ip` 从 prior state 的 `null` 变成 planned unknown。

## 测试资源

- Terraform 资源：`volcenginecc_vpc_eni`
- Cloud Control 类型：`Volcengine::VPC::ENI`
- Set 属性：`PrivateIpSets`
- 业务 identity：`PrivateIpAddress`
- 非 identity computed 字段：`AssociatedElasticIp`

配置中的业务元素为：

```hcl
private_ip_sets = [
  {
    private_ip_address = "192.168.0.221"
  }
]
```

同一资源还显式配置：

```hcl
tags = []
```

## 第二次 Plan 证据

`second-plan.json` 中目标 ENI 的值为：

```text
before.private_ip_sets[0].associated_elastic_ip = null
after_unknown.private_ip_sets[0].associated_elastic_ip = true
before.tags = null
after.tags = []
```

第二次 plan 输出：

```text
SECOND_PLAN_EXIT_CODE=2
Plan: 0 to add, 1 to change, 0 to destroy.
```

Framework debug 日志记录：

```text
marking computed attribute that is null in the config as unknown
tf_attribute_path="AttributeName(\"private_ip_sets\")...AttributeName(\"associated_elastic_ip\")"
```

## 因果边界

本 case 证明：

> `SetNestedAttribute` 可以在配置完全未修改的第二次 plan 中出现非 identity
> 字段 `null -> unknown`，并把同一个业务元素显示成 Set remove/add。

触发规划的第一处稳定差异是：

```text
config tags = []
remote/state tags = null
```

该 `null/empty` 差异使整个资源满足
`PlannedState != PriorState`，Framework 随后执行 computed-null-to-unknown
标记，令 `AssociatedElasticIp` 从 `null` 变成 unknown。

因此，本 case 不能证明 Set 内的 `null -> unknown` 在没有任何其他规划差异时会
独立启动第二次 plan；它证明的是：无需用户修改配置，只要同一资源存在其他稳定
差异，Set 内非 identity computed null 就可能在每次 plan 中被重新放大为
remove/add。

## 证据文件

- `main.tf`
- `second-plan.log`
- `second-plan.json`
- `provider-debug.log`

