# VEPFS AttachFileSystems Read 反序候选验证结果

> 日期：2026-06-29  
> 分支：`terraform-identity-validation`  
> 目录：`workspace/unordered_array_identity_research/read_reorder/negative_cases/vepfs_mount_service`

## 目标

验证 `Volcengine::VEPFS::MountService.AttachFileSystems` 是否满足阶段 E 真实云验收条件：

- 生成形态为 `ListNestedAttribute + generic.Multiset()`；
- 元素存在用户可知、服务端稳定读回的 identity 候选；
- 真实 `UpdateResource` 反序提交后，`GetResource` 能保留不同于 config/prior 的顺序；
- 若服务端保留反序，再验证修复后 Read 能按 identity 对齐并收敛。

## 候选条件

`AttachFileSystems` 的静态条件较好：

- Cloud Control schema 为 `insertionOrder=false`、`uniqueItems=false`；
- 生成 Terraform schema 为 `ListNestedAttribute + generic.Multiset()`；
- 元素内 `CustomerPath`、`FileSystemId` 均为 required；
- `Status`、`AccountId`、`FileSystemName` 是服务端 readback 字段；
- `CustomerPath + FileSystemId` 可以作为用户可知、服务端稳定读回的 identity 候选。

## 执行过程

本轮尝试创建完整真实资源链路：

1. 创建两个临时 `volcenginecc_vepfs_instance`：
   - `tf-vepfs-read-reorder-a`
   - `tf-vepfs-read-reorder-b`
2. 创建一个 `volcenginecc_vepfs_mount_service`，并在
   `attach_file_systems` 中挂载上述两个文件系统。
3. 若创建成功，再执行 `reorder_vepfs_attach_file_systems.go`，通过
   `UpdateResource` replace `/AttachFileSystems` 反序并检查 `GetResource`。

## 结果

两个 `Volcengine::VEPFS::Instance` 创建任务均在云侧失败，未进入 MountService 创建：

```text
ErrorCode: ServiceInternalError
TypeName: Volcengine::VEPFS::Instance
Operation: CREATE
OperationStatus: FAILED
TaskID: task-c457306f-f3b2-4e43-8083-404dcb554843
StatusMessage: ServiceInternalError: InternalError: Service has some internal Error. Pls Contact With Admin.
```

第二个文件系统同样失败：

```text
TaskID: task-526c5c0a-a9dc-4734-9c3c-268052463fb2
StatusMessage: ServiceInternalError: InternalError: Service has some internal Error. Pls Contact With Admin.
```

随后用原生 Go SDK 直接调用 VEPFS `CreateFileSystem` 做对照验证：

```sh
go run ./workspace/unordered_array_identity_research/read_reorder/negative_cases/vepfs_mount_service/create_vepfs_instance_sdk.go
```

只读探针 `DescribeFileSystems` 成功，说明 VEPFS SDK endpoint、签名和凭证链路可用：

```text
describe probe ok: request_id=20260629234914249611D26DC68AAA67D4-6ccb4d count=0
```

但同参数创建文件系统仍失败：

```text
create file system failed: InternalError: Service has some internal Error. Pls Contact With Admin.
status_code=500
request_id=20260629234915CB8A9AA70C2554DE602A-d3f62c
code=InternalError
message=Service has some internal Error. Pls Contact With Admin.
```

## 资源适配仓库定位

本地补充查看了 `~/Code/resource-vepfs-instance`，它是
`Volcengine::VEPFS::Instance` 的 Cloud Control 资源适配仓库。

关键链路如下：

- `resource/create_resource.go` 的 `createInfo` 直接调用
  `svc.CreateFileSystemWithContext(ctx, transformModel2CreateRequest(resourceModel))`；
- `resource/transform.go` 的 `transformModel2CreateRequest` 只是把 schema model 映射到
  VEPFS SDK `CreateFileSystemInput`，其中 `ProjectName` 映射到 SDK 字段 `Project`；
- `resource/model/error_map.go` 把 SDK 返回的 `InternalError` 映射为 Cloud Control
  `ErrServiceInternalError`，所以 Terraform 侧看到的 `ServiceInternalError` 是这一层映射后的结果；
- 仓库自带 `inputs/inputs_1_create.json` 与本轮参数基本一致，核心差异是样例使用
  `Capacity: 6`，本轮 Terraform/SDK 验证使用过 `Capacity: 8`。

当前沙箱网络受限，尝试用 `VEPFS_CAPACITY=6` 复跑原生 SDK 时停在 DNS 层：

```text
lookup open.volcengineapi.com: no such host
```

该复跑不是服务端响应，不能作为容量 6 成功或失败的云侧证据。

## 结论

`VEPFS.MountService.AttachFileSystems` 仍是静态上最干净的候选之一，但本账号/区域当前无法创建
前置 `VEPFS::Instance`，因此本轮不能完成真实云反序验收。

Go SDK 原生 API 的 `DescribeFileSystems` 可用而 `CreateFileSystem` 同样返回服务端 500，
说明失败不局限在 Cloud Control 翻译层，更像 VEPFS 创建侧或账号/区域环境侧阻塞。
结合 `resource-vepfs-instance` 仓库，Cloud Control 资源适配层没有明显额外加工或改写创建参数；
它基本就是透传到 VEPFS Go SDK。
这不是 identity 证据失败，也不是服务端已证明会规范排序；它是环境/云侧创建能力阻塞。
后续如果能提供两个现成可挂载的 VEPFS 文件系统，或者云侧修复 `VEPFS::Instance` 创建失败，
可以直接复用本目录的 `main.tf` 和 `reorder_vepfs_attach_file_systems.go` 继续验证。

## 清理

创建失败后执行：

```sh
TF_CLI_CONFIG_FILE=/private/tmp/terraform-provider-volcenginecc-dev.tfrc terraform state list
```

`logs/phase-e-vepfs-final-state-list.log` 为空，确认本 case 没有 Terraform 资源残留。

Go SDK 创建在返回 `FileSystemId` 前失败，没有产生可删除资源；输出见
`logs/phase-e-vepfs-go-sdk-create.log`。

## 2026-07-01：`volc-eps-ccapi-test` profile 重试

按用户指定，使用 `~/.volcengine/config.json` 中的
`volc-eps-ccapi-test` profile 重跑原生 VEPFS SDK 只读探针：

```sh
VOLCENGINE_PROFILE=volc-eps-ccapi-test \
VEPFS_PROBE_ONLY=1 \
go run ./workspace/unordered_array_identity_research/read_reorder/negative_cases/vepfs_mount_service/create_vepfs_instance_sdk.go
```

请求已到达 VEPFS 服务，但 `DescribeFileSystems` 返回：

```text
InvalidAccessKey
```

该 profile 当前只有 `access-key`、`secret-key` 两个字段，没有 session token
或过期时间元数据。服务端错误显示该 access key 无效，因此本轮没有进入
`CreateFileSystem`，也没有创建任何云资源。

本次结果不能用来判断 `volc-eps-ccapi-test` 身份是否具备 VEPFS 创建能力；
需要先刷新该 profile 的有效凭证。如果它使用临时 AK/SK，还需同时保存对应的
session token，再重跑本目录探针。

### 补充 session token 后重试

用户补充 `session-token` 后再次执行同一只读探针。profile 结构已确认包含且非空：

```text
access-key
secret-key
session-token
```

字段名与 Go SDK `clicreds` 支持的格式一致，但 VEPFS 服务返回：

```text
InvalidSecretToken
```

服务端明确判定该 session token 不属于当前 access key。因此阻塞从“缺少有效临时
凭证”收敛为“AK/SK/session token 不是同一次签发的完整 STS 三件套”。需要一起
替换为同一次签发得到的三项值，仅补 token 不能继续创建验证。

### 更换完整 STS 三件套后重试

用户更换完整凭证后，`volc-eps-ccapi-test` profile 的 VEPFS
`DescribeFileSystems` 探针成功：

```text
describe probe ok
```

为排除旧 VPC/Subnet 属于其他身份的问题，本轮使用该 profile 真实创建了临时 VPC
和 Subnet，并让两个 `Volcengine::VEPFS::Instance` 引用同账号网络资源。容量使用
资源适配仓库样例值 `6`。

结果是两个 Cloud Control 创建任务仍同时失败：

```text
TypeName: Volcengine::VEPFS::Instance
Operation: CREATE
ErrorCode: ServiceInternalError
StatusMessage: InternalError: Service has some internal Error.
```

随后使用同一 profile、同一临时 VPC/Subnet、同一容量 6，通过原生 VEPFS SDK
执行对照：

```text
DescribeFileSystems: success
CreateFileSystem: HTTP 500 InternalError
```

因此本轮进一步排除了：

1. 原 profile 凭证失效；
2. STS 三件套不匹配；
3. VPC/Subnet 跨账号；
4. Capacity 8 与资源样例 Capacity 6 的差异；
5. Cloud Control resource handler 单独引入错误。

阻塞仍位于 VEPFS `CreateFileSystem` 服务侧或该账号的产品开通/配额能力，
尚未进入 MountService 创建和 AttachFileSystems 反序验证。

本轮临时 VPC、Subnet 已成功销毁；两个 VEPFS 创建均未返回 FileSystemId，
Terraform state 最终为空。

### 移除已下线的 `VersionNumber=1.4.0` 后完整验证

VEPFS 服务维护方确认 `VersionNumber: "1.4.0"` 已不支持创建。该字段在 schema
和原生 API 中不是必填，因此本轮移除显式 `VersionNumber`，让服务端选择当前默认
版本。

移除后，两个文件系统不再立即返回 500，而是成功创建：

```text
tf-vepfs-read-reorder-a: Running, VersionNumber=1.5.0
tf-vepfs-read-reorder-b: Running, VersionNumber=1.5.0
```

随后 `Volcengine::VEPFS::MountService` 也成功创建，并稳定读回两个
`AttachFileSystems`：

```text
0: /tf-vepfs-read-reorder-a
1: /tf-vepfs-read-reorder-b
```

通过真实 Cloud Control `UpdateResource` 提交完整反序 `[B,A]` 后，再次
`GetResource` 仍返回：

```text
0: /tf-vepfs-read-reorder-a
1: /tf-vepfs-read-reorder-b
```

因此最终结论更新为：

1. 之前的 `InternalError` 根因是已下线的 `VersionNumber=1.4.0`，服务端没有返回
   清晰的参数错误；
2. 省略版本后服务端默认使用 `1.5.0`，VEPFS Instance 和 MountService 均可创建；
3. `AttachFileSystems` 的 `/CustomerPath + /FileSystemId` 仍是良好 identity
   候选；
4. 但服务端会把反序规范回原顺序，无法制造阶段 E 所需的 remote reorder；
5. VEPFS 应归入“identity 明确但服务端 canonical ordering”的排除类，不再是环境
   创建阻塞类。

本轮 MountService、两个 VEPFS Instance、Subnet 和 VPC 共 5 个临时资源均已成功
销毁，Terraform state 为空。
