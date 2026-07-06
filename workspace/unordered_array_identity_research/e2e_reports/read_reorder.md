# Read remote 反序 E2E 测试记录

> 本文依次整理问题复现与 identity-aware Read 对齐的真实验收记录。

## 一、问题复现：ListNestedAttribute + generic.Multiset

> 旧文档来源：[`e2e_list_nested_attribute_multiset_unordered_result.md`](../e2e_reports_backup/e2e_list_nested_attribute_multiset_unordered_result.md)，原文开头至“五、Read/Plan 风险复现”。

> 测试用例目录：[`autoscaling_scaling_configuration/`](../read_reorder/autoscaling_scaling_configuration/)  
> 测试资源：`Volcengine::AutoScaling::ScalingConfiguration.Volumes`

### 1. 验证目标

验证 `README.md` 对 `insertionOrder=false` 无序 object 数组提出的两个判断：

1. CCAPI Read 返回不同顺序时，`ListNestedAttribute + generic.Multiset()`
   是否仍会产生 reorder-only plan；
2. current 与 planned 顺序不同时，Update JSON Patch 是否按数组下标重写
   多个元素。

本轮目标字段：

```text
Volcengine::AutoScaling::ScalingConfiguration.Volumes
```

对应 Terraform 类型：

```text
schema.ListNestedAttribute
  + generic.Multiset()
```

### 2. 真实测试链路

```text
Terraform v1.15.4
  -> 当前 main 构建的 volcenginecc Provider
  -> 真实生成 schema
  -> genericResource Create / Read / Update
  -> Cloud Control API
  -> cn-beijing 真实云资源
  -> GetResource readback
  -> Terraform Core plan
  -> Provider patchDocument
  -> UpdateResource
```

没有使用 mock Provider，也没有手工构造 Terraform 协议值。

认证使用：

```text
~/.volcengine/config.json
profile = default
```

AK/SK 没有写入测试目录、日志或文档。

### 3. 资源与成本控制

本轮创建：

- 临时 VPC；
- 临时 Subnet；
- 临时 SecurityGroup；
- 临时 ECS LaunchTemplate；
- 临时 ECS LaunchTemplateVersion；
- `min=0/max=0` 的临时 AutoScaling ScalingGroup；
- 临时 AutoScaling ScalingConfiguration。

整个测试没有创建 ECS 实例。

测试目录：

```text
workspace/unordered_array_identity_research/read_reorder/autoscaling_scaling_configuration
```

### 4. 初始状态

Terraform 配置：

```hcl
volumes = [
  {
    delete_with_instance = true
    size                 = 40
    volume_type          = "ESSD_PL0"
  },
  {
    delete_with_instance = true
    size                 = 50
    volume_type          = "ESSD_PL0"
  }
]
```

创建后 CCAPI GetResource：

```text
Volumes = [40, 50]
```

### 5. Read/Plan 风险复现

通过真实 Cloud Control `UpdateResource` 将完整 `Volumes` 反序提交：

```text
[40, 50] -> [50, 40]
```

随后真实 GetResource 保留：

```text
Volumes = [50, 40]
```

main.tf 没有修改，再次执行 Terraform plan，实际显示：

```text
Volumes[0].Size: 50 -> 40
Volumes[1].Size: 40 -> 50
```

退出码：

```text
PLAN_EXIT_CODE=2
```

结论：

> `generic.Multiset()` 没有让这个真实 readback 乱序收敛。稳定配置产生了
> reorder-only plan，README 描述的 Read drift 已复现。

## 二、修复验收：AutoScaling Volumes identity 链路

> 旧文档来源：[`e2e_read_reorder_autoscaling_e1_result.md`](../e2e_reports_backup/e2e_read_reorder_autoscaling_e1_result.md)，全文。

> 测试用例目录：[`autoscaling_scaling_configuration/`](../read_reorder/autoscaling_scaling_configuration/)  
> 测试资源：`Volcengine::AutoScaling::ScalingConfiguration.Volumes`

### 1. 验收边界

本轮只验收 identity metadata 到真实 Read 对齐、plan 收敛的完整链路，不把
`Size + VolumeType` 认定为生产合规 identity。合法配置中该组合可能重复，后续仍需
业务侧确认或补充稳定 key。

### 2. 测试对象

```text
Volcengine::AutoScaling::ScalingConfiguration.Volumes
elementIdentifier = ["/Size", "/VolumeType"]
uniqueItems = false
insertionOrder = false
Terraform type = ListNestedAttribute + generic.Multiset()
```

测试配置使用两个 identity 不重复的元素：

```text
[40 GiB ESSD_PL0, 60 GiB ESSD_PL0]
```

### 3. 真实链路

```text
Terraform v1.15.4
-> 当前 terraform-identity-validation 分支本地 Provider
-> Cloud Control cn-beijing
-> 真实 ScalingConfiguration
-> Cloud Control UpdateResource 反序
-> GetResource 保留反序
-> Provider Read identity 对齐
-> Terraform plan JSON
```

ScalingGroup 使用 `min=0/max=0`，没有创建 ECS 实例。

### 4. 结果

创建后的 Terraform state 顺序：

```text
[40, 60]
```

Cloud Control 反序并再次 GetResource：

```text
[60, 40]
```

随后执行真实 Terraform plan，目标资源结果：

```json
{
  "address": "volcenginecc_autoscaling_scaling_configuration.identity",
  "actions": ["no-op"],
  "before_volumes": [40, 60],
  "after_volumes": [40, 60]
}
```

整份 plan 的退出码为 2，但唯一变化来自
`volcenginecc_autoscaling_scaling_group.identity` 的既有
`launch_template_version` 默认值 drift；目标 ScalingConfiguration 明确为
`no-op`。

### 5. 结论

阶段 E1 已打通：

1. schema `elementIdentifier` 能进入生成代码和 runtime metadata；
2. Cloud Control 能稳定保留不同于 config/prior 的真实顺序；
3. 未修复基线已经证明该顺序产生 reorder-only drift；
4. 修复后 Read 能按受控 identity 对齐；
5. 目标资源的后续 plan 收敛为 `no-op`。

阶段 E2 仍未完成：`Size + VolumeType` 可能重复，不能据此宣布 AutoScaling
`Volumes` 已具备生产合规 identity。

### 6. 清理

本轮创建的 ScalingConfiguration、ScalingGroup、LaunchTemplateVersion、
LaunchTemplate、SecurityGroup、Subnet 和 VPC 均已删除，最终 Terraform state
为空。
