# Multiset 稳定 Plan `null -> unknown` 触发原因重跑结果

## 结论

使用 `origin/main@fe912e8600d5c09f239d54efe4c9bd1db10d1cb5`
重新创建真实 ECS 后，协议证据表明：

```text
PlanResourceChange Request PriorState
==
PlanResourceChange Request ProposedNewState
```

两者完整值比较结果：

```text
equal=true diff_count=0
```

因此，该 case 不是 Terraform Core 在 `ProposedNewState` 中先制造了差异。

Framework 收到请求后，默认值处理阶段改变了 `eip_address`。Framework debug
日志记录的 unknown marking 前唯一顶层差异是：

```text
Detected value change between proposed new state and prior state:
tf_attribute_path=eip_address
```

紧接着才执行：

```text
Marking Computed attributes with null configuration values as unknown
```

## `eip_address` 为什么变化

配置没有声明 `eip_address`。Read 后的 PriorState 和 Core 生成的
ProposedNewState 中，它都是一个已知 object，内部字段为 null。

生成 schema 为两个内部字段声明了默认值：

```go
charge_type = "PayByBandwidth"
release_with_instance = false
```

Framework 从 `ProposedNewState` 初始化 `PlannedState` 后执行
`TransformDefaults()`，默认值使 `eip_address` 与 PriorState 首次不同，满足：

```text
PlannedState != PriorState
```

随后 `MarkComputedNilsAsUnknown()` 遍历整个 ECS，把与该默认值变化无关的
computed null 也标记为 unknown，包括：

```text
secondary_network_interfaces[0].ipv_6_addresses:
null -> unknown

secondary_network_interfaces[0].private_ip_addresses:
null -> unknown
```

字段级 `UseStateForUnknown()` 因 prior value 为 null 而不恢复；最后
`generic.Multiset()` 使用完整 object `Equal()`，因此不能把 planned list
收敛回 prior list。

## Write-only 排除结论

Framework debug 日志中没有
`Nullifying write-only attribute in the newState`。

当前生成 schema 也没有设置 Framework 原生 `WriteOnly: true`；Cloud Control
write-only 信息只以注释和 generic resource metadata 存在。因此本 case 的
`NullifyWriteOnlyAttributes()` 没有制造初始差异。

## 最终链路

```text
Core: PriorState == ProposedNewState
  -> Framework TransformDefaults
  -> eip_address 内部默认值使 PlannedState != PriorState
  -> MarkComputedNilsAsUnknown
  -> SecondaryNetworkInterfaces 非 identity 字段 null -> unknown
  -> 字段级 UseStateForUnknown 因 prior=null 不恢复
  -> generic.Multiset 完整元素 Equal 失败
  -> 稳定第二次 plan 退出码 2
```

## 证据

- `protocol-cause-rerun/`
- `logs/cause-rerun-prior-vs-proposed-2.txt`
- `logs/cause-rerun-provider-debug.log`
- `logs/cause-rerun-prior-vs-planned.txt`
- `logs/cause-rerun-stable-plan.json`
- `decode_plan_diff.go`

