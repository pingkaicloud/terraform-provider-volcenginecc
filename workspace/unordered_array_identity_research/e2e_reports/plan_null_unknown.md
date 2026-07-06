# Plan `null -> unknown` E2E 测试记录

> 本文按 Terraform collection 表达分别整理 Set 与 Multiset 的真实 E2E 记录。

## 一、SetNestedAttribute：`null -> unknown`

> 旧文档来源：[`e2e_set_plan_value_diff_result.md`](../e2e_reports_backup/e2e_set_plan_value_diff_result.md)，原文“二、`null -> unknown`”。

> 测试用例目录：[`vpc_eni_description_update/`](../plan_null_unknown/vpc_eni_description_update/)  
> 测试资源：`Volcengine::VPC::ENI.PrivateIpSets`

### 1. 测试资源

测试资源：

```text
Volcengine::VPC::ENI.PrivateIpSets
```

Terraform 表达：

```text
schema.SetNestedAttribute
```

配置只填写辅助私网 IP：

```hcl
private_ip_sets = [
  {
    private_ip_address = "192.168.0.220"
  }
]
```

`associated_elastic_ip` 是元素内的 `Optional + Computed`
`SingleNestedAttribute`，未绑定 EIP 时 state 为：

```text
associated_elastic_ip = null
```

### 2. 稳定基线

首次 apply 成功后执行 plan：

```text
No changes. Your infrastructure matches the configuration.
PLAN_EXIT_CODE=0
```

说明仅有 `state=null` 并不会自动产生 drift。稳定配置下 Core 已把 prior state
合入 proposed state，Framework 没有把该字段重新标记为 unknown。

### 3. 触发一次真实顶层更新

只修改 ENI 顶层 Description：

```text
"Set null to unknown baseline"
-> "Set null to unknown trigger"
```

`private_ip_sets` 配置完全未变，但 plan 中出现：

```text
~ private_ip_sets = [
    - {
        - private_ip_address = "192.168.0.220" -> null
      },
    + {
        + associated_elastic_ip = (known after apply)
        + private_ip_address    = "192.168.0.220"
      },
  ]
```

plan JSON 精确记录：

```text
before.associated_elastic_ip       = null
after_unknown.associated_elastic_ip = true
```

Framework debug log 同时记录：

```text
marking computed attribute that is null in the config as unknown
```

这说明同一业务元素仅因非 identity 字段从 `null` 变为 `unknown`，完整 set object
身份发生变化，Terraform 将其表现为 remove/add。

### 4. Apply 与 PatchDocument

真实 apply 成功。Provider 生成的 PatchDocument 只有真正修改的顶层字段：

```json
[
  {
    "op": "replace",
    "path": "/Description",
    "value": "Set null to unknown trigger"
  }
]
```

没有包含 `PrivateIpSets`。apply 后再次执行稳定 plan：

```text
No changes. Your infrastructure matches the configuration.
PLAN_EXIT_CODE=0
```

因此本 case 证明的是：

- `null -> unknown` 可以污染 `SetNestedAttribute` 的元素匹配和 plan 展示；
- 当前 generic update 过滤没有把这条伪差异发给 CCAPI；
- 当前尚未复现不伴随其他变化、稳定配置仍持续出现的独立 drift。

测试目录：

```text
../plan_null_unknown/vpc_eni_description_update/
```

## 二、ListNestedAttribute + generic.Multiset：`null -> unknown`

> 旧文档来源：[`e2e_plan_non_identity_result.md`](../e2e_reports_backup/e2e_plan_non_identity_result.md)，原文“三、Multiset：每个节点的状态”至“四、执行命令”。

> 测试用例目录：[`ecs_instance/`](../plan_null_unknown/ecs_instance/)  
> 测试资源：`Volcengine::ECS::Instance.SecondaryNetworkInterfaces`

实际执行顺序：

```text
步骤 1：terraform apply，创建安全组、ECS 和辅助网卡
步骤 2：Cloud Control Read，确认远端辅助网卡字段
步骤 3：查看 apply 后的 Terraform state
步骤 4：main.tf 不变，再次执行 terraform plan
步骤 5：查看 plan 的 before、after 和 after_unknown
步骤 6：terraform destroy，清理临时资源
```

### 节点 M0：main.tf 配置

配置一个辅助网卡元素：

```hcl
secondary_network_interfaces = [
  {
    subnet_id          = "subnet-rrwqhg3qzxfkv0x57g3edcq"
    security_group_ids = ["sg-2f8jctjkrw4qo4f4pzzs31fp3"]
  }
]
```

用户没有配置：

```text
NetworkInterfaceId
MacAddress
PrimaryIpAddress
VpcId
Ipv6Addresses
PrivateIpAddresses
```

其中前四项由服务端生成或补充；这些字段不用于表达“用户想要哪个辅助网卡配置”。

### 节点 M1：执行真实 apply

执行：

```bash
terraform apply -auto-approve -no-color
```

真实创建安全组、ECS 和辅助网卡后，Cloud Control Read 返回：

```text
Ipv6AddressCount  = 0
MacAddress        = 00:16:3e:4a:09:97
NetworkInterfaceId = eni-3hj21lrblyzuo3nkipjhoxx46
PrimaryIpAddress  = 192.168.0.187
SecurityGroupIds  = [sg-2f8jctjkrw4qo4f4pzzs31fp3]
SubnetId          = subnet-rrwqhg3qzxfkv0x57g3edcq
VpcId             = vpc-rrco37ovjq4gv0x58zft8ul
```

Cloud Control 没有返回：

```text
Ipv6Addresses
PrivateIpAddresses
```

### 节点 M2：apply 后的 Terraform state

Provider 将真实 readback 翻译为：

```text
ipv_6_address_count  = 0
ipv_6_addresses      = null
mac_address          = 00:16:3e:4a:09:97
network_interface_id = eni-3hj21lrblyzuo3nkipjhoxx46
primary_ip_address   = 192.168.0.187
private_ip_addresses = null
security_group_ids   = [sg-2f8jctjkrw4qo4f4pzzs31fp3]
subnet_id            = subnet-rrwqhg3qzxfkv0x57g3edcq
vpc_id               = vpc-rrco37ovjq4gv0x58zft8ul
```

此时远端与 state 是一致的：

| Cloud Control Read | Terraform state |
|-|-|
| 字段省略 | 对应值为 `null` |
| 服务端生成 NIC ID/MAC/IP/VPC | state 保存对应值 |
| Subnet/SG 与配置一致 | state 保存对应值 |

### 节点 M3：main.tf 未变，再次执行 plan

执行：

```bash
terraform plan -out=stable.tfplan -detailed-exitcode -no-color
terraform show -json stable.tfplan > stable-plan.json
```

用户没有修改 main.tf。Terraform 为同一个元素生成 planned value 时：

```text
ipv_6_addresses      = unknown
private_ip_addresses = unknown
```

而 prior state 中对应值仍为：

```text
ipv_6_addresses      = null
private_ip_addresses = null
```

完整对比：

| 字段 | prior state | proposed plan | identity 是否变化 |
|-|-|-|-|
| `subnet_id` | 原值 | 原值 | 否 |
| `security_group_ids` | 原值 | 原值 | 否 |
| `network_interface_id` | 原值 | 原值 | 否 |
| `mac_address` | 原值 | 原值 | 否 |
| `vpc_id` | 原值 | 原值 | 否 |
| `ipv_6_addresses` | `null` | `unknown` | 否，非 identity 字段 |
| `private_ip_addresses` | `null` | `unknown` | 否，非 identity 字段 |

### 节点 M4：generic.Multiset() 比较

当前实现对每个完整 object 调用：

```go
currentVal.Equal(plannedVal)
```

它没有只比较 `SubnetId + SecurityGroupIds` 等业务身份，而是比较完整元素。

本 case 中：

```text
null != unknown
```

所以完整 object 不相等，`generic.Multiset()` 没有把 prior list 保留为
planned list。

### 节点 M5：最终 plan

最终 plan 中，`secondary_network_interfaces` 出现：

```text
~ secondary_network_interfaces = [
    ~ {
        + ipv_6_addresses      = (known after apply)
        + private_ip_addresses = (known after apply)
      }
  ]
```

整个 ECS 资源结果：

```text
Plan: 0 to add, 1 to change, 0 to destroy.
STABLE_PLAN_EXIT_CODE=2
```

状态链路：

```text
main.tf 未变化
        |
        v
远端仍是同一张辅助网卡
        |
        v
state 中两个非 identity 字段为 null
        |
        v
proposed plan 中两个字段为 unknown
        |
        v
完整 object Equal 失败
        |
        v
Multiset 没有保留 prior list
        |
        v
辅助网卡字段出现无意义 diff
```

证据边界：

> ECS 整体 plan 还同时包含其他顶层 Optional+Computed 字段的
> `(known after apply)`。本轮可以精确证明
> `SecondaryNetworkInterfaces` 自身没有被 `generic.Multiset()` 收敛，并在
> plan 中产生了无意义 diff；不能把 ECS 资源的所有 plan 差异都归因于
> Multiset。

### 节点 M6：清理

执行：

```bash
terraform destroy -auto-approve -no-color
terraform state list
```

临时 ECS、辅助网卡和安全组均销毁成功，`terraform state list` 输出为空。

## 三、执行命令

### 1. 构建当前 Provider

```bash
go build \
  -o /private/tmp/tf-volcenginecc-e2e-provider/terraform-provider-volcenginecc \
  ./main.go
```

### 2. SetNestedAttribute

> 测试用例目录：[`negative_cases/vpc_security_group/`](../plan_null_unknown/negative_cases/vpc_security_group/)  
> 测试资源：`Volcengine::VPC::SecurityGroup.IngressPermissions`

```bash
cd workspace/unordered_array_identity_research/plan_null_unknown/negative_cases/vpc_security_group
export TF_CLI_CONFIG_FILE="$PWD/terraform-cli.tfrc"

terraform apply -auto-approve -no-color
terraform plan -detailed-exitcode -no-color

go run change_and_restore_ingress.go \
  "$(terraform output -raw security_group_id)"

terraform plan -detailed-exitcode -no-color
terraform destroy -auto-approve -no-color
terraform state list
```

### 3. ListNestedAttribute + generic.Multiset()

> 测试用例目录：[`ecs_instance/`](../plan_null_unknown/ecs_instance/)  
> 测试资源：`Volcengine::ECS::Instance.SecondaryNetworkInterfaces`

```bash
cd workspace/unordered_array_identity_research/plan_null_unknown/ecs_instance
export TF_CLI_CONFIG_FILE="$PWD/terraform-cli.tfrc"

terraform apply -auto-approve -no-color
terraform plan -out=stable.tfplan -detailed-exitcode -no-color
terraform show -json stable.tfplan > stable-plan.json

terraform destroy -auto-approve -no-color
terraform state list
```
