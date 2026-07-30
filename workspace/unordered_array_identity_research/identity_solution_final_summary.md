# 无序对象数组 identity 最终总结

> 初稿日期：2026-06-28  
> 总结日期：2026-07-04  
> 规范排序修订：2026-07-28
> 分支：`terraform-identity-validation`  
> 分支快照：`4ff7048 fix: canonicalize nested identity collections`<br>
> 基准：只读飞书镜像 `README.md`  
> 证据：当前目录下真实 Plan / Read / Update E2E 结果及修复验收结果  
> 状态：分支实现及 E2E 验收总结

## 一、结论

`README.md` 确立的主线仍然成立：

> CCAPI schema 声明无序数组元素的 `elementIdentifier`，Provider 在 Read、
> Plan、Update 和 WriteOnly 回填阶段使用统一的 identity 元数据和匹配语义。

这里的“统一”不表示四个阶段直接调用同一个函数。当前实现分别使用：

- Plan：`mergeIdentityCollectionPlans()` + `canonicalizeIdentityCollectionPlan()`；
- Read：`canonicalizeIdentityCollectionState()`；
- Update：`canonicalizeIdentityCollections()`；
- WriteOnly：`restoreIdentityCollectionWriteOnlyValues()`。

四条路径共享 `collectionIdentities` 元数据、identity 提取规则以及“不按 index
猜测”的安全边界，但各自根据阶段职责处理 Terraform value、JSON desired state
或 writeOnly 回填。

2026-07-28 修订后，排序不再以 prior、planned 或 Provider 某次 GetResource 的
remote 顺序为标准。所有阶段统一采用 elementIdentifier tuple 的客观规范升序；
CCAPI Handler 也必须在实际应用 JSON Patch 前对最新资源文档执行同一排序。
字符串按大小写敏感的 UTF-8 字节序，数字按精确数值，布尔值按
`false < true`。

真实 E2E 进一步证明 Plan 阶段也必须接入 identity：

> Provider 在 Plan 阶段按 identity 配对 prior/planned 元素，再根据字段
> 所有权和规范化语义处理非 identity 字段。

完整方案因此分为两层：

```text
第一层：elementIdentifier
  回答“左右两边是不是同一个逻辑元素”

第二层：字段级语义
  回答“同一元素中的字段差异是否应该进入 plan / patch”
```

identity 不是 ignored fields 列表。非 identity 字段不等于可以忽略的字段。

## 二、已复现 E2E 案例、问题原因与修复

| 已复现的 E2E 案例 | 问题原因 | 如何修复 |
|-|-|-|
| Plan：Set / Multiset 的 computed/readback 字段 `null -> unknown`。<br />测试文档：[Plan `null -> unknown` E2E 测试记录](https://www.feishu.cn/docx/Wke2dizsVot9swxQzCIc4OAlnee) | Framework 在资源存在其他规划差异时会把 computed null 标记为 unknown。Set 使用完整 object 判定元素身份，`generic.Multiset()` 也只做完整 object `Equal()`；二者都不知道变化前后是同一个逻辑元素。 | 为 collection 声明 `elementIdentifier`；Plan 阶段先按业务 identity 配对 prior/planned 元素，再根据 Config 所有权合并字段。配置未声明的 computed/readback 字段保留 prior value，用户显式配置的变化不应被吞掉。 |
| Read：Multiset 的 remote 反序产生 reorder-only plan。 <br>测试文档：[Read remote 反序 E2E 测试记录](https://www.feishu.cn/docx/X1H1dlwHMoROzNxyLBncApftnKg) | Read 直接接受 remote 顺序；当元素中还存在 readback 字段差异时，`generic.Multiset()` 无法靠完整 object 相等恢复稳定顺序。 | Read 阶段按 `elementIdentifier` 规范升序输出完整 remote state，不依赖 prior 顺序；Plan merge 继续作为字段语义保护。 |
| Update：Provider GetResource 与 CCAPI Handler 实际应用 Patch 时数组顺序可能不同。 | 旧修复以 Provider 的 `remoteDesiredState` 为下标基准，只能保证 Provider 本地 Patch 正确，不能约束 Handler 的第二次快照。 | Provider current/planned 分别按 canonical identity order 排序；CCAPI Handler 对实际 Patch base 执行同一排序后再应用 Patch。 |

方案必须同时覆盖这些样本，不能只让某一个 plan 变成 `No changes`。

用户显式空字符串与服务端具体回填值之间的等价性属于字段语义问题，不由本分支
处理。ECS `PrimaryIpAddress` 的历史方案和证据已移至
[`UseStateForEmpty` 字段语义问题独立记录](use_state_for_empty_solution.md)。

## 三、实现总览

| 组件 | 当前实现 | 目标 |
|-|-|-|
| Schema 元数据 | 已实现 | 为已确认的无序 object array 声明 `elementIdentifier`；字段级 readOnly/writeOnly/default/规范化语义仍分别维护。 |
| 公共 identity 匹配层 | 已实现 | 提取组合 identity、检查缺失/unknown/重复、配对 collection elements。 |
| Plan identity-aware merge | 已实现 | 消除非 identity computed/readback 字段造成的无意义 diff，同时保留真实配置变化。 |
| Plan canonical order | 已实现 | identity-aware merge 后，对已知 identity 的 PlannedState 执行规范排序。 |
| Read canonical order | 已实现 | remote 不依赖 prior，按 elementIdentifier 规范升序进入 state。 |
| Create plan canonical order | 已实现 | ModifyPlan 只排序已知 identity；Create 请求和 response 不二次重排，避免服务端补齐 unknown identity 后交换 List 元素。 |
| Provider Update canonical order | 已实现 | current/planned 独立规范排序后生成 JSON Patch。 |
| CCAPI Update canonical order | 待 CCAPI 仓库实现 | 对 Handler 实际 Patch base 使用同一 comparator 排序。 |
| WriteOnly identity-aware 回填 | 已实现 | 按 identity 从 prior state 回填 writeOnly 字段。 |
| 风险保护 | 已实现于已声明 identity 的 collection | identity 不安全时不猜测、不按 index 静默处理；无安全 identity 的 Multiset 全局策略仍未完成。 |

## 四、Schema 元数据

### 1. `elementIdentifier`

本分支沿用 `README.md` 中的定义：

```json
"Labels": {
  "type": "array",
  "insertionOrder": false,
  "uniqueItems": true,
  "elementIdentifier": ["/Name"],
  "items": {
    "$ref": "#/definitions/TemplateKV"
  }
}
```

路径相对于单个数组元素，支持组合 identity：

```json
"elementIdentifier": [
  "/Role",
  "/CustomerGatewayId"
]
```

identity 字段必须满足：

```text
readable ∩ configurable ∩ stable ∩ unique
```

- 排除 readOnly 和 writeOnly 字段；
- createOnly 字段可以使用，但 Read 必须稳定返回；
- identity 为 null/unknown 时不能匹配；
- 组合 identity 仍重复时不能任选一个候选。

### 2. 字段级语义

`elementIdentifier` 只负责匹配。是否抑制字段差异仍由已有 schema 信息和明确的
规范化规则决定：

| 字段类型 | 默认所有权 |
|-|-|
| Required / 普通 Optional | 用户配置所有 |
| Computed-only / 严格 readOnly | 服务端所有 |
| Optional + Computed，配置为 null | 服务端可以回填 |
| Optional + Computed，用户显式配置 | 用户声明了期望值 |
| writeOnly | 用户配置所有，但 Read 时需要从 prior state 恢复 |

对于服务端会改写的字段，需要三选一：

1. 字段实际只读：修正 `readOnlyProperties`；
2. 输入值非法：增加 validator，在 apply 前拒绝；
3. 两种表示业务等价：增加字段级 canonical normalizer 或 semantic equality。

不能用“它不是 identity 字段”为理由忽略用户显式值。

## 五、公共 identity 匹配层

本分支新增：

```text
internal/generic/collection_identity.go
```

核心输入：

```go
type CollectionIdentity struct {
    PropertyPath    string
    IdentifierPaths []string
    UniqueItems     bool
}
```

核心能力：

1. 根据 collection path 找到目标数组；
2. 按 JSON Pointer 从元素中提取组合 identity；
3. 使用类型和值均稳定的编码生成比较 key；
4. 检查 identity 缺失、null、unknown；
5. 检查同一集合中 identity 重复；
6. 建立 prior/config/planned/remote 之间的元素配对；
7. 返回配对结果，而不是在 helper 内直接决定忽略哪些字段。

内部配对结果的核心元素结构为：

```go
type collectionElementPair struct {
    Identity   collectionElementIdentityKey
    LeftIndex  int
    RightIndex int
}
```

关键行为：

- Set 与 Multiset 共用同一套配对逻辑；
- identity 不唯一时返回明确结果，不回退到 index；
- 元素新增、删除和 identity 改变必须保留；
- 没有 `elementIdentifier` 的现有资源默认不改变行为。

## 六、Plan identity-aware merge

### 1. 接入点

`genericResource` 实现：

```text
resource.ResourceWithModifyPlan
```

处理链：

```text
Terraform Core 生成 ProposedNewState
  -> Framework defaults / unknown marking / attribute plan modifiers
  -> genericResource.ModifyPlan
  -> 按 identity 配对 prior/planned 元素
  -> 按字段级语义合并
  -> 最终 PlannedState
```

### 2. 字段合并规则

对 identity 已匹配的元素：

| 字段状态 | Plan 行为 |
|-|-|
| identity 字段变化 | 不合并，保留真实元素变化。 |
| 配置未声明，planned 为 unknown，prior 为已知值或 null | planned 使用 prior。 |
| Computed-only / 严格 readOnly | planned 使用 refreshed prior。 |
| Optional + Computed，配置为 null | planned 优先使用 prior。 |
| Optional + Computed，用户显式配置 | 保留 planned，不自动覆盖。 |
| Required / 普通 Optional | 保留 planned。 |
| 有明确 canonical normalizer | 比较或写入 canonical value。 |

### 3. 对现有样本的预期

ENI case：

```text
identity:
  PrivateIpAddress = "192.168.0.220"

prior:
  AssociatedElasticIp = null

planned:
  AssociatedElasticIp = unknown
```

`PrivateIpAddress` 相同，且 `AssociatedElasticIp` 未在 config 声明，因此合并后：

```text
planned.AssociatedElasticIp = null
```

最终不再显示整个 `PrivateIpSets` 元素 remove/add。

SecurityGroup `Direction` case 不应被该逻辑吞掉：

```text
config.Direction = ""
state.Direction  = "ingress"
```

因为用户显式配置了 `Direction`。正确处理位置是 schema/validator/normalizer，
不是通用 identity merge。

### 4. `ObjectSemanticEquals()` 的位置

不将 `ObjectSemanticEquals()` 作为主方案：

- 它一次只能比较两个 object，不能独立解决组合 identity 重复和全局配对；
- 返回语义相等时会保留 prior value，不能单独完成“remote 完整进入 state”；
- 它不能处理 Update JSON Patch 和 WriteOnly 回填。

可以保留为少量字段级规范化的补充，但主路径是公共 identity helper +
`ModifyPlan`。

## 七、Read canonical order

目标不是丢弃 remote 字段，而是：

```text
remote 完整 readback 进入 tfstate
+ remote 元素按 elementIdentifier tuple 的规范升序排列
+ 顺序变化不制造无意义 plan
```

处理流程：

```text
remote collection
  -> 按 JSON Pointer 提取单字段、组合或嵌套 identity
  -> 检查缺失、null、类型和重复
  -> 按规范 comparator 排序
  -> 完整 remote object 写入 state
```

Set 在 Terraform 层没有顺序语义，因此纯 remote 反序不会形成 reorder-only plan。
Set 的主要风险是 readback/default 字段改变完整 object 身份，并在后续 Plan 表现为
元素 remove/add；该问题主要由 Plan identity-aware merge 处理。Multiset 使用 List
表示，remote 顺序则会直接影响 state 是否稳定，因此 Read canonical order 对
Multiset 是独立且必要的保护。

## 八、Provider 与 CCAPI Update canonical order

修复前 Update 链路：

```text
prior Terraform state
  -> toCloudControl.AsString
  -> GetResource
  -> mergeLocalWithRemoteForSets
  -> translateForUpdate
  -> patchDocument(current, planned)
```

主要缺口：

- `mergeLocalWithRemoteForSets()` 遇到 Set 直接采用 remote；
- List 内部仍按 index 递归；
- 没有 schema 声明的业务 identity；
- `patchDocument()` 最终仍按数组下标生成 patch。

Provider 当前实现链路：

```text
prior/current Terraform state
planned desired
remote desired state
  -> mergeLocalWithRemoteForSets
  -> current 和 planned 分别按同一 elementIdentifier 规范升序排列
  -> 保留新增/删除元素
  -> 对齐 writeOnly 值
  -> patchDocument(canonicalCurrent, canonicalPlanned)
```

CCAPI Handler 必须补齐：

```text
UpdateResource 实际使用的最新资源文档
  -> 使用同一 schema elementIdentifier 和 comparator 规范排序
  -> 在 canonical resource document 上应用 Provider PatchDocument
```

Provider 与 Handler 只要面对的是同一成员集合，原始数组顺序如何变化都不会改变
identity 的 canonical index。该保证不覆盖两次快照之间的并发新增/删除；这类成员
变化仍需要锁、资源版本或条件更新。

验收要求：

- remote 只反序时 PatchDocument 为空；
- Provider/Handler 两次 GetResource 顺序不同时仍命中正确 identity；
- 修改一个逻辑元素时，只生成目标元素 patch；
- identity 改变时保留真实删除/新增；
- identity 不唯一时不生成可能改错对象的局部 index patch。

## 九、WriteOnly 回填

本分支已将 identity collection 内的 writeOnly 回填改为：

```text
prior element identity
  -> planned/remote 同 identity element
  -> 回填该元素的 writeOnly 字段
```

identity 缺失、null、unknown 或重复时：

- 不按 index 猜测；
- 返回明确 diagnostic；
- 必要时要求整字段替换、改模或拆成子资源。

## 十、代码接入链

```text
CCAPI schema
  array.elementIdentifier
        |
        v
internal/ccschema.Property
  ElementIdentifier []string
        |
        v
generator template data / schema.tmpl
        |
        v
generated *_resource_gen.go
  opts.WithCollectionIdentities(...)
        |
        v
genericResource.collectionIdentities
        |
        +--> ModifyPlan
        +--> Read reorder
        +--> Update normalize
        +--> WriteOnly refill
```

| 层 | 当前文件 | 已实现内容 |
|-|-|-|
| Schema 解析 | `internal/ccschema/property.go` | `Property` 增加 `ElementIdentifier []string`。 |
| Schema 校验 | `internal/ccschema/collection_identity.go` | 限制其用于 `insertionOrder=false` 的 object array，校验 pointer 指向可用的 element 字段。 |
| 生成器 | `internal/provider/generators/shared/template_data.go` | 收集 collection path、identity pointer、uniqueItems。 |
| 模板 | `internal/provider/generators/resource/schema.tmpl` | 生成 `opts.WithCollectionIdentities(...)`。 |
| Runtime option | `internal/generic/resource.go` | `genericResource` 保存 `collectionIdentities`，四阶段从同一份 metadata 进入。 |
| 公共 helper | `internal/generic/collection_identity.go` | `pairCollectionsByIdentity()` 负责 identity 提取、唯一性检查和元素配对；同文件实现 JSON desired state normalize。 |
| Plan | `internal/generic/resource.go`、`internal/generic/collection_identity_plan.go` | `ModifyPlan()` 调用 `mergeIdentityCollectionPlans()` 做字段级 merge。 |
| Plan | `internal/generic/resource.go`、`internal/generic/collection_identity_plan.go` | identity-aware merge 后由 `canonicalizeIdentityCollectionPlan()` 排序已知 identity。 |
| Read | `internal/generic/resource.go`、`internal/generic/collection_identity_plan.go` | remote 写 state 前由 `canonicalizeIdentityCollectionState()` 规范排序。 |
| Create | `internal/generic/resource.go`、`internal/generic/collection_identity_plan.go` | 复用 ModifyPlan 已产生的顺序；Create 请求和 response 不额外排序。 |
| Update | `internal/generic/resource.go`、`internal/generic/collection_identity.go` | `patchDocument()` 前由 `canonicalizeIdentityCollections()` 独立排序 current/planned。 |
| WriteOnly | `internal/generic/resource.go`、`internal/generic/collection_identity_write_only.go` | 按 identity 从 prior 回填后保持 canonical order。 |

不要手改生成的 `internal/volcengine/*/*_resource_gen.go`。元数据必须从 schema parser
和 generator 进入生成代码。

## 十一、测试矩阵

| 场景 | 预期 |
|-|-|
| Set 非 identity computed 字段 `null -> unknown` | identity 相同且配置未声明该字段时，planned 保留 prior，不出现元素 remove/add。 |
| 非 identity readOnly/readback 字段变化 | remote 进入 state，但不产生用户不可操作的 diff。 |
| 用户显式配置普通字段 | 差异必须保留，不能被 identity merge 吞掉。 |
| identity 字段变化 | 保留真实元素删除/新增。 |
| identity 为 null/unknown | 不匹配、不按 index 猜测。 |
| 组合 identity 重复 | 返回明确 diagnostic。 |
| Set remote 反序 | Set 本身无序，纯顺序变化不应形成 reorder-only plan；readback/default 导致的完整 object 身份变化由 Plan 用例覆盖。 |
| Multiset remote 反序 | Read 按 canonical identity order 写入完整 remote 元素，重复 plan 无 reorder-only diff。 |
| Update 前 GetResource 反序 | 不生成 reorder-only patch。 |
| Provider GetResource 与 Handler GetResource 顺序不同 | 两边规范排序后，Patch 仍命中同一 identity。 |
| 单元素业务修改 | 只生成目标元素 patch。 |
| Multiset 允许完整重复元素 | identity 能区分时处理；不能区分时降级。 |
| writeOnly + remote 反序 | 按 identity 回填到正确元素。 |
| 无 `elementIdentifier` 的现有资源 | 保持现有行为，避免全仓无条件改变。 |

## 十二、实际验收范围

### 1. 已通过的真实 E2E

| 阶段 | 真实资源与场景 | 验收结果 |
|-|-|-|
| Plan / Set | VPC ENI `PrivateIpSets` 的 `AssociatedElasticIp null -> unknown` | description-only 变更不再带出 `private_ip_sets` remove/add，最终稳定 plan 收敛。 |
| Plan / Multiset | ECS `SecondaryNetworkInterfaces` 的未配置 computed/readback 字段 `null -> unknown` | 目标 collection before/after 保持一致，identity-aware merge 不再制造无意义 replacement。 |
| Read / Multiset | AutoScaling `ScalingConfiguration.Volumes` remote 保留反序 | 修复前产生 reorder-only plan；修复后目标资源为 `no-op`。该用例使用的 `/Size + /VolumeType` 仅是受控测试 identity。 |
| Update / Set | VPC PrefixList `PrefixListEntries` remote 为 `[B, A]` 后修改 B | 修复前 patch index 指向 A；修复后 patch 指向 remote index 0 的 B，A 不变，最终 plan 收敛。 |

详细命令、云端 readback、plan JSON、patch 和清理证据分别见本目录：

- [Plan `null -> unknown` E2E 测试记录](https://www.feishu.cn/docx/Wke2dizsVot9swxQzCIc4OAlnee)；
- [Read remote 反序 E2E 测试记录](https://www.feishu.cn/docx/X1H1dlwHMoROzNxyLBncApftnKg)；
- [Update index patch E2E 测试记录](https://www.feishu.cn/docx/MtvBdSQtkoLW3ixSO2NcHLJgnhe)。

### 2. 自动化证据

分支包含以下层次的测试：

- schema 解析、JSON Pointer 和 readOnly/writeOnly identity 校验；
- generator template data 与生成代码编译检查；
- typed identity、组合 identity、反序、增删、缺失、null、unknown、重复测试；
- Framework `PlanResourceChange` 协议路径测试；
- Read 对齐、Update normalize、WriteOnly 回填 focused tests；
- ENI 生成资源级 Plan 回归测试。

`internal/generic` 完整 package test 曾保留 6 个与本任务无关的既有 `arn`/`trn`
fixture 失败，因此不能将“全仓所有测试通过”写成本分支结论；本任务新增和受影响
的 focused tests、compile-only 检查及上述真实 E2E 均有独立证据。

### 3. 未完成与不能扩大解释的边界

1. SecurityGroup `Direction "" -> "ingress"` 的字段所有权仍待 schema/API
   维护方确认，通用 identity merge 不会忽略用户显式配置。
2. AutoScaling `Volumes` 的 `/Size + /VolumeType` 在合法配置中可能重复，仅用于
   打通受控 Read E2E，不能作为生产合规 identity 直接交付。
3. ECS `SecondaryNetworkInterfaces`、ENI `PrivateIpSets` 和 VPN
   `TunnelOptions` 的反序提交会被服务端恢复规范顺序，不能作为 remote 保留反序的
   Read E2E；这不否定它们在 Plan 或其他阶段的证据。
4. 无安全 `elementIdentifier` 的 Multiset 仍缺少全局整字段替换、warning、
   computed-only、改模或子资源策略。本分支只保证已声明 identity 的 collection
   不会在 identity 缺失、null、unknown 或重复时按 index 静默猜测。
5. 本分支没有对全仓所有无序 object array 批量启用 identity；metadata 只添加到
   已分析并用于验证的目标资源。

## 十三、最终结论

`terraform-identity-validation` 已证明：`elementIdentifier` 可以从 CCAPI schema
贯通到生成代码和 runtime，并在 Plan、Read、Update、WriteOnly 四个阶段形成
可执行的 identity-aware 处理链。目标 E2E 已分别证明该链路能够处理完整 object
身份不稳定、remote 反序以及 JSON Patch 下标错位等问题。

该结论只适用于具有安全、稳定、可读且唯一 identity 的 collection。identity
负责确认“是不是同一个逻辑元素”，字段所有权、服务端规范化以及无安全 identity
时的改模策略仍是独立问题，不能由通用 identity helper 自动推断。
