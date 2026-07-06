# Update index patch E2E 测试记录

> 本文依次整理 Multiset 风险复现、Set 错元素更新复现与 identity-aware Update normalize 验收记录。

## 一、Multiset：Update 多下标 patch 风险复现

> 旧文档来源：[`e2e_list_nested_attribute_multiset_unordered_result.md`](../e2e_reports_backup/e2e_list_nested_attribute_multiset_unordered_result.md)，原文开头至“四、初始状态”及“六、Update 下标风险复现”。

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

### 5. Update 下标风险复现

在远端仍为 `[50,40]` 时，用户只把配置中的 `50 GiB` 修改为 `60 GiB`：

```text
current remote = [50, 40]
planned config = [40, 60]
```

Terraform plan 显示：

```text
Volumes[0].Size: 50 -> 40
Volumes[1].Size: 40 -> 60
```

Provider 真实生成的 PatchDocument：

```json
[
  {
    "op": "replace",
    "path": "/Volumes/0/Size",
    "value": 40
  },
  {
    "op": "replace",
    "path": "/Volumes/1/Size",
    "value": 60
  }
]
```

用户只修改了一个逻辑元素，但 Provider 重写了两个数组下标。

Update 后远端成为：

```text
[40, 60]
```

最终 plan：

```text
No changes.
FINAL_PLAN_EXIT_CODE=0
```

本 case 最终业务值正确，但它是通过重写两个 index 达到的。当前 Provider
不知道“哪个 remote Volume 对应配置中的哪个 Volume”，只能同时修正顺序和
业务值。

## 二、Set：PrefixList 错元素更新复现

> 旧文档来源：[`e2e_set_nested_attribute_unordered_result.md`](../e2e_reports_backup/e2e_set_nested_attribute_unordered_result.md)，原文“四、补充验证：PrefixList `PrefixListEntries`”。

> 测试用例目录：[`vpc_prefix_list/`](../update_index_patch/vpc_prefix_list/)  
> 测试资源：`Volcengine::VPC::PrefixList.PrefixListEntries`

### 1. 为什么选择 PrefixList

`PrefixListEntries` 是 `uniqueItems=true, insertionOrder=false` 的 object 数组，
生成代码为 `schema.SetNestedAttribute`。元素里的 `Cidr` 在 Cloud Control schema
的 `PrefixListEntry.required` 中；在 Terraform schema 里生成成
`Optional + Computed + NotNullString`，不是 Terraform `Required` 字段。

本 case 仍可把 `/Cidr` 作为候选 identity，原因是：

1. 用户可以在配置中声明 `cidr`；
2. `GetResource` 会稳定返回 `Cidr`；
3. CIDR 是前缀列表条目的业务 key，同一前缀列表内不应重复；
4. `Description` 可作为非 identity 修改字段，用来验证 Update 是否命中正确元素。

配置中写入两条条目：

| 逻辑元素 | CIDR | Description |
|-|-|-|
| A | `10.252.0.0/25` | `identity-entry-a` |
| B | `10.252.0.128/25` | `identity-entry-b` |

证据文件：`../update_index_patch/vpc_prefix_list/main.tf`

### 2. 初始 apply 与 baseline plan

执行：

```bash
TF_CLI_CONFIG_FILE="$PWD/terraform-cli.tfrc" terraform apply -auto-approve -no-color
terraform plan -detailed-exitcode -no-color
```

结果：

```text
volcenginecc_vpc_prefix_list.identity: Creation complete after 6s [id=pl-2f8zo0u2nbx1c4f4q005osmic]
No changes. Your infrastructure matches the configuration.
BASELINE_PLAN_EXIT_CODE=0
```

证据文件：

- `../update_index_patch/vpc_prefix_list/logs/apply.log`
- `../update_index_patch/vpc_prefix_list/logs/baseline-plan.log`

说明：Terraform 稳定配置可以收敛，但这不代表服务端顺序和配置书写顺序一致。

### 3. 真实 `GetResource` 已返回反序

helper 读取 Cloud Control `GetResource` 后，初始远端顺序已经是：

```text
before:
0: Cidr=10.252.0.128/25 object=map[Cidr:10.252.0.128/25 Description:identity-entry-b]
1: Cidr=10.252.0.0/25 object=map[Cidr:10.252.0.0/25 Description:identity-entry-a]
```

也就是说，配置顺序是 `[A, B]`，但真实 `GetResource` 返回 `[B, A]`。这和
SecurityGroup `IngressPermissions` 不同：PrefixList 是一个服务端真实保留或产生
反序 readback 的 Set object case。

证据文件：`../update_index_patch/vpc_prefix_list/logs/reinsert-entry.log`

### 4. 删除后重新追加，服务端仍保留 `[B, A]`

为了避免只靠初始创建顺序下结论，本轮又做了“移除后追加”的真实更新：

```text
patch: [{"op":"remove","path":"/PrefixListEntries/0"}]
after remove:
0: Cidr=10.252.0.0/25 object=map[Cidr:10.252.0.0/25 Description:identity-entry-a]

patch: [{"op":"add","path":"/PrefixListEntries/-","value":{"Cidr":"10.252.0.128/25","Description":"identity-entry-b"}}]
after re-add:
0: Cidr=10.252.0.128/25 object=map[Cidr:10.252.0.128/25 Description:identity-entry-b]
1: Cidr=10.252.0.0/25 object=map[Cidr:10.252.0.0/25 Description:identity-entry-a]
```

结果说明：删除 B 后只剩 A；把 B 追加回去后，服务端最终仍返回 `[B, A]`。
这确认 PrefixList 在本轮真实链路中可以复现并维持
“remote 顺序不同于 config/prior 顺序”的状态，足以作为 Update 下标风险的
验收条件。

证据文件：`../update_index_patch/vpc_prefix_list/logs/reinsert-entry.log`

### 5. 临时加入 `/Cidr` identity 后，Read 仍可收敛

为了确认新增 identity metadata 不会破坏现有 Set 收敛，本轮临时给
`Volcengine::VPC::PrefixList.PrefixListEntries` 增加：

```json
"elementIdentifier": ["/Cidr"]
```

然后重新生成当前 Provider、重建 `/tmp/tf-volcenginecc-dev/terraform-provider-volcenginecc`。
这只是本地验证改动，测试后已回退，没有提交。

在真实 remote 仍为 `[B, A]` 时执行 plan：

```text
No changes. Your infrastructure matches the configuration.
IDENTITY_READ_PLAN_EXIT_CODE=0
```

debug 日志同时记录：

```text
Cloud Control API GetResource:
"PrefixListEntries":[
  {"Cidr":"10.252.0.128/25","Description":"identity-entry-b"},
  {"Cidr":"10.252.0.0/25","Description":"identity-entry-a"}
]
```

而最终 `Response.State.Raw` 中的 `prefix_list_entries` 仍可与 prior/config 收敛。
但这个结果不能证明 `/Cidr` identity 对 Set Read 收敛有增量效果，因为
`schema.SetNestedAttribute` 本身就会忽略纯顺序差异：只要元素完整内容相同，
`[A, B]` 与 `[B, A]` 对 Terraform 都是同一个 set。

因此本 case 的 Read 结论只能写成：

1. 真实 remote 顺序确实是 `[B, A]`；
2. 临时加入 `/Cidr` identity 后没有破坏 Set 的既有收敛；
3. 这不是 identity-aware Read reorder 的强证明。

证据文件：

- `../update_index_patch/vpc_prefix_list/logs/identity-read-plan.log`
- `../update_index_patch/vpc_prefix_list/logs/identity-read-debug.log`

### 6. 单元素 Update 复现错元素更新

随后只修改逻辑元素 B：

```hcl
{
  cidr        = "10.252.0.128/25"
  description = "identity-entry-b-updated"
}
```

plan 显示只想替换 B 这个 set 元素：

```text
prefix_list_entries = [
  - {
      cidr        = "10.252.0.128/25"
      description = "identity-entry-b"
    },
  + {
      cidr        = "10.252.0.128/25"
      description = "identity-entry-b-updated"
    },
  # (1 unchanged element hidden)
]
UPDATE_DESCRIPTION_PLAN_EXIT_CODE=2
```

证据文件：`../update_index_patch/vpc_prefix_list/logs/update-description-plan.log`

但 apply debug 日志中的真实 Cloud Control patch 是：

```json
[
  {
    "op": "replace",
    "path": "/PrefixListEntries/1/Description",
    "value": "identity-entry-b-updated"
  }
]
```

证据文件：`../update_index_patch/vpc_prefix_list/logs/update-description-debug.log`

问题在于，真实远端顺序是：

```text
0: B = 10.252.0.128/25
1: A = 10.252.0.0/25
```

所以 `/PrefixListEntries/1/Description` 实际命中的是 A，而不是用户想修改的 B。

最终 plan 证实了错元素更新：

```text
prefix_list_entries = [
  - {
      cidr        = "10.252.0.0/25"
      description = "identity-entry-b-updated"
    },
  - {
      cidr        = "10.252.0.128/25"
      description = "identity-entry-b"
    },
  + {
      cidr        = "10.252.0.0/25"
      description = "identity-entry-a"
    },
  + {
      cidr        = "10.252.0.128/25"
      description = "identity-entry-b-updated"
    },
]
FINAL_PLAN_EXIT_CODE=2
```

debug 日志中的最终 `GetResource` 也直接显示：

```text
"PrefixListEntries":[
  {"Cidr":"10.252.0.128/25","Description":"identity-entry-b"},
  {"Cidr":"10.252.0.0/25","Description":"identity-entry-b-updated"}
]
```

也就是 B 仍是旧描述，A 被错误写成了 B 的新描述。

证据文件：

- `../update_index_patch/vpc_prefix_list/logs/update-description-apply.log`
- `../update_index_patch/vpc_prefix_list/logs/update-description-debug.log`
- `../update_index_patch/vpc_prefix_list/logs/final-plan.log`
- `../update_index_patch/vpc_prefix_list/logs/final-plan-debug.log`

### 7. PrefixList 清理

执行：

```bash
terraform destroy -auto-approve -no-color
terraform state list
```

结果：

```text
Destroy complete! Resources: 1 destroyed.
```

`terraform state list` 输出为空。

证据文件：

- `../update_index_patch/vpc_prefix_list/logs/destroy.log`
- `../update_index_patch/vpc_prefix_list/logs/state-list-after-destroy.log`

## 三、修复验收：PrefixList Update normalize

> 旧文档来源：[`e2e_update_normalize_prefix_list_result.md`](../e2e_reports_backup/e2e_update_normalize_prefix_list_result.md)，全文。

> 测试用例目录：[`vpc_prefix_list/`](../update_index_patch/vpc_prefix_list/)  
> 测试资源：`Volcengine::VPC::PrefixList.PrefixListEntries`

### 1. 验收对象

```text
Volcengine::VPC::PrefixList.PrefixListEntries
Terraform type = SetNestedAttribute
elementIdentifier = ["/Cidr"]
```

配置中的逻辑顺序：

```text
A = 10.252.0.0/25
B = 10.252.0.128/25
```

真实 Cloud Control `GetResource` 顺序：

```text
0: B
1: A
```

本轮只把 B 的 Description 从 `identity-entry-b` 修改为
`identity-entry-b-updated`。

### 2. 首次验收暴露的实现缺陷

仅加入 `/Cidr` metadata 后，阶段 F 旧实现仍生成：

```json
[
  {
    "op": "replace",
    "path": "/PrefixListEntries/1/Description",
    "value": "identity-entry-b-updated"
  }
]
```

由于真实 remote index 1 是 A，该 patch 错误修改了 A。最终 GetResource 为：

```text
0: B / identity-entry-b
1: A / identity-entry-b-updated
```

最终 plan 退出码为 2。

根因是 `mergeLocalWithRemoteForSets()` 合并 remote 信息后仍保留 Terraform 本地
set 的迭代顺序。旧 `normalizeIdentityCollections()` 只把 current 对齐 planned，
没有使用 patch 真正作用的 remote 数组顺序，因此两边仍按 `[A,B]` 生成 index patch。

### 3. 修复

`normalizeIdentityCollections()` 新增真实 `remoteDesiredState` 输入，并把 current
和 planned 的匹配元素都按 remote identity 顺序排列。新增回归测试直接覆盖：

```text
remote = [B,A]
current/planned = [A,B]
只修改 B
```

测试要求 patch 必须命中 `/PrefixListEntries/0/Description`，并把 patch 应用到
remote JSON 后确认只有 B 被修改。

### 4. 修复后真实结果

重新创建 PrefixList 后，GetResource 再次返回：

```text
0: B / identity-entry-b
1: A / identity-entry-a
```

Terraform apply 的真实 Cloud Control patch：

```json
[
  {
    "op": "replace",
    "path": "/PrefixListEntries/0/Description",
    "value": "identity-entry-b-updated"
  }
]
```

最终 GetResource：

```text
0: B / identity-entry-b-updated
1: A / identity-entry-a
```

最终稳定 plan：

```text
No changes. Your infrastructure matches the configuration.
FIXED_FINAL_PLAN_EXIT_CODE=0
```

### 5. 结论

阶段 F 的真实验收已通过：

1. `/Cidr` identity metadata 已进入生成代码和 runtime；
2. 服务端真实返回不同于 config/prior 的 `[B,A]`；
3. 未修复时已复现 index 1 改错 A；
4. 修复后 patch 根据 remote identity 顺序命中 index 0；
5. B 正确更新、A 保持不变；
6. 最终 plan 收敛为 0。

测试 PrefixList 已销毁，Terraform state 为空。
