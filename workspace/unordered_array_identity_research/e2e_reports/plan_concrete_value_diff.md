# Plan 非 identity 具体值差异 E2E 测试记录

> 本文按 Terraform collection 表达分别整理 Set 与 Multiset 的真实 E2E 记录。

## 一、SetNestedAttribute：非 identity 具体值差异

> 旧文档来源：[`e2e_set_plan_value_diff_result.md`](../e2e_reports_backup/e2e_set_plan_value_diff_result.md)，原文“三、非 identity 具体值差异”至“四、额外观察”。

> 测试用例目录：[`vpc_security_group/`](../plan_concrete_value_diff/vpc_security_group/)  
> 测试资源：`Volcengine::VPC::SecurityGroup.IngressPermissions`

### 1. 创建基线资源

测试资源：

```text
Volcengine::VPC::SecurityGroup.IngressPermissions
```

第一次执行 `terraform apply` 时，ingress rule 没有填写 `direction`：

```hcl
ingress_permissions = [
  {
    description = "set-null-unknown-ssh"
    policy      = "accept"
    port_start  = 22
    port_end    = 22
    priority    = 20
    protocol    = "tcp"
    cidr_ip     = "10.250.0.0/25"
  }
]
```

执行：

```bash
terraform apply -auto-approve
terraform plan -detailed-exitcode
```

结果：

```text
Apply complete! Resources: 1 added, 0 changed, 0 destroyed.
No changes. Your infrastructure matches the configuration.
PLAN_EXIT_CODE=0
```

Cloud Control 和 Terraform state 中的真实 readback 为：

```text
direction = "ingress"
```

至此基线资源创建成功，且基线配置可以收敛。

`Direction` 用于描述规则方向，不是本 case 中识别 SSH 规则的业务 identity。

### 2. 修改配置后执行第一次 plan

在基线资源仍然存在的情况下，修改 `main.tf`，给同一条 ingress rule 显式增加：

```hcl
direction = ""
```

此时只修改了配置，尚未执行第二次 apply。先执行：

```bash
terraform plan -detailed-exitcode \
  -out=logs/concrete-empty-direction.tfplan
```

Terraform plan 将整个 set 元素显示为删除后新增：

```text
- 原 set 元素：direction = "ingress"
+ 新 set 元素：direction = ""
```

plan JSON 对应为：

```text
before.direction = "ingress"
after.direction  = ""
```

这里的 remove/add 是“修改 `main.tf` 后、执行第二次 apply 之前”的第一次 plan。
单看这一步还不能证明 drift，因为此时用户确实修改了配置。

### 3. 执行第二次 apply

将上一步保存的 plan 真正应用：

```bash
terraform apply -auto-approve \
  logs/concrete-empty-direction.tfplan
```

结果：

```text
Modifications complete after 11s
Apply complete! Resources: 0 added, 1 changed, 0 destroyed.
```

Cloud Control 接受了更新，但服务端没有保留空字符串，远端真实值仍为：

```text
remote.direction = "ingress"
```

第二次 apply 刚结束时，Provider 返回的 planned value 被写入本地
`terraform.tfstate`：

```text
main.tf           = ""
terraform.tfstate = ""
remote            = "ingress"
```

### 4. 配置不再变化，执行第二次 plan

第二次 apply 完成后，不再修改 `main.tf`，保持：

```hcl
direction = ""
```

再次执行：

```bash
terraform plan -detailed-exitcode \
  -out=logs/concrete-empty-direction-stable.tfplan
```

这条命令会先执行 refresh。Provider 从 Cloud Control 读回远端真实值后，本次
plan 内部使用的 refreshed state 为：

```text
refreshed state.direction = "ingress"
```

这里要区分 plan 内存中的 refreshed state 与磁盘上的 `terraform.tfstate`：
普通 `terraform plan` 使用刷新结果计算差异，但不会把它持久化回本地 state 文件。
因此第二次 plan 计算时的关系是：

```text
main.tf                = ""
refreshed remote/state = "ingress"
```

结果仍然显示整个 set 元素删除后新增：

```text
Plan: 0 to add, 1 to change, 0 to destroy.
PLAN_EXIT_CODE=2
```

plan JSON 仍然是：

```text
before.direction = "ingress"
after.direction  = ""
```

真正证明 drift 的是这一次 plan：第二次 apply 已经成功，之后配置没有再发生变化，
但 plan refresh 再次读取到服务端规范化后的 `"ingress"`，Terraform 因而仍然计划
更新同一个资源。差异不是 object 内的单字段更新，而是整个 set 元素 remove/add。

测试目录：

```text
../plan_concrete_value_diff/vpc_security_group/
```

### 5. 额外观察

使用 SecurityGroup minimal rule，同时显式配置一条 egress rule 时，服务端还会
额外增加两条默认 egress rule。当前 Provider apply 后真实报错：

```text
Provider produced inconsistent result after apply
actual set element ... does not correlate with any element in plan
egress_permissions: length changed from 1 to 3
```

这是另一种 Set 元素/集合形状变化证据，但混入了 remote-only 默认元素，不作为
本轮 `null -> unknown` 的主证据。

## 二、ListNestedAttribute + generic.Multiset：非 identity 具体值差异

> 旧文档来源：[`e2e_multiset_plan_concrete_value_result.md`](../e2e_reports_backup/e2e_multiset_plan_concrete_value_result.md)，全文。

> 测试用例目录：[`ecs_instance/`](../plan_concrete_value_diff/ecs_instance/)  
> 测试资源：`Volcengine::ECS::Instance.SecondaryNetworkInterfaces`

### 1. 验证目标

之前已复现：

```text
state   = null
planned = unknown
```

本轮继续验证：在 main.tf 不变时，`ListNestedAttribute + generic.Multiset()`
能否出现某个具体值的 diff。

测试字段：

```text
Volcengine::ECS::Instance.SecondaryNetworkInterfaces
```

Terraform 表达：

```text
schema.ListNestedAttribute + generic.Multiset()
```

### 2. 执行步骤与每个节点的状态

#### 节点 C0：main.tf

辅助网卡配置：

```hcl
secondary_network_interfaces = [
  {
    subnet_id          = "subnet-rrwqhg3qzxfkv0x57g3edcq"
    security_group_ids = [volcenginecc_vpc_security_group.plan.security_group_id]
    primary_ip_address = ""
  }
]
```

这里 `primary_ip_address = ""` 表示用户没有指定固定私网 IP，由服务端自动分配。
`PrimaryIpAddress` 是 createOnly 字段，但不是可稳定识别该数组元素的 identity。

#### 节点 C1：执行 apply

```bash
terraform apply -auto-approve -no-color
```

真实结果：

```text
Apply complete! Resources: 2 added, 0 changed, 0 destroyed.
ECS ID = i-yepdjk4h6owh2yqm9y4o
```

此时 main.tf 没有再修改。

#### 节点 C2：apply 后的本地 state

apply 刚完成时：

```text
primary_ip_address = ""
```

完整辅助网卡 state：

```text
network_interface_id = eni-w0txojlkdvy8865yk9xjz4di
mac_address          = 00:16:3e:76:36:68
primary_ip_address   = ""
subnet_id            = subnet-rrwqhg3qzxfkv0x57g3edcq
vpc_id               = vpc-rrco37ovjq4gv0x58zft8ul
```

#### 节点 C3：执行稳定配置 plan

main.tf 保持不变，执行：

```bash
terraform plan -out=final-stable.tfplan -detailed-exitcode -no-color
```

plan 开始时先执行 refresh。真实 Cloud Control Read 返回服务端自动分配的具体
私网 IP：

```text
primary_ip_address = "192.168.0.191"
```

此时：

| 对象 | `primary_ip_address` |
|-|-|
| main.tf / planned | `""` |
| refresh 后的 remote/state | `"192.168.0.191"` |
| Subnet、SecurityGroup 等业务配置 | 没有变化 |

#### 节点 C4：Multiset 完整元素比较

`generic.Multiset()` 使用完整 object：

```go
currentVal.Equal(plannedVal)
```

其中一个具体字段已经不同：

```text
current primary_ip_address = "192.168.0.191"
planned primary_ip_address = ""
```

因此完整元素 Equal 失败，Multiset 没有保留 prior list。

#### 节点 C5：最终 plan

Terraform CLI 显示：

```text
- primary_ip_address = "192.168.0.191" -> null # forces replacement
```

CLI 把空字符串显示成了 `null`，但 plan JSON 明确记录 planned value 为具体空字符串：

```json
{
  "actions": ["delete", "create"],
  "before": [
    {
      "primary_ip_address": "192.168.0.191"
    }
  ],
  "after": [
    {
      "primary_ip_address": ""
    }
  ],
  "replace_paths": [
    [
      "secondary_network_interfaces",
      0,
      "primary_ip_address"
    ]
  ]
}
```

最终结果：

```text
Plan: 1 to add, 0 to change, 1 to destroy.
FINAL_STABLE_PLAN_EXIT_CODE=2
```

这不是 unknown diff，而是：

```text
具体值 "192.168.0.191" -> 具体值 ""
```

并且它触发了整台 ECS replacement。如果执行 replacement，新的辅助网卡仍会由
服务端自动分配具体 IP，因此该配置存在重复出现同类 replacement plan 的风险。

#### 节点 C6：A→B 补充尝试

为了验证能否制造两个非空具体 IP 的归一化，曾将配置改为：

```hcl
primary_ip_address = "0.0.0.0"
```

真实 Cloud Control Create 拒绝该值：

```text
InvalidIp: The specified ip is not valid, is unsupported, or cannot be used.
```

因此没有把失败尝试当作复现证据，也没有据此创建 ECS。

#### 节点 C7：清理

```bash
terraform destroy -auto-approve -no-color
terraform state list
```

临时 ECS、辅助网卡和安全组均已销毁，`terraform state list` 输出为空。

### 3. 正式结论

`ListNestedAttribute + generic.Multiset()` 已真实复现非 identity 字段的具体值
diff：

```text
remote/state = "192.168.0.191"
planned      = ""
```

main.tf 在 apply 后没有修改，Subnet 和 SecurityGroup 配置没有变化，但完整元素
Equal 被 `PrimaryIpAddress` 影响，最终产生整台 ECS replacement plan。

因此当前已有两类 Plan 证据：

| 类型 | 是否复现 |
|-|-|
| `null -> unknown` | 已复现 |
| 具体值 `"192.168.0.191" -> ""` | 已复现，并触发 replacement |

### 4. 证据文件

测试目录：

```text
../plan_concrete_value_diff/ecs_instance/
```

关键证据：

- `logs/final-apply-empty-input.log`
- `logs/final-state-after-apply.json`
- `logs/final-stable-plan.log`
- `logs/final-stable-plan-secondary-change.json`
- `logs/final-destroy.log`
- `logs/final-state-list-after-destroy.log`
