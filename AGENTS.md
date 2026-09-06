# Terraform Provider 开发事项

本文档面向后续维护 `terraform-provider-volcenginecc` 的开发者。该仓库是 Volcengine Cloud Control Terraform Provider，也是 `provider-upjet-volcengine` 的底层 Terraform Provider。修改前应先理解这两个使用场景：直接由 Terraform 调用，以及由 Upjet 在 Crossplane 控制器中嵌入调用。

## 1. 仓库定位与边界

- Terraform Provider 的模块路径和 Go package 名称保持 `github.com/volcengine/terraform-provider-volcenginecc` / `volcengine`。不要改成 PingCAP、Pingkai 或其他内部名称，这样可以继续同步官方上游代码，并保持 Terraform source `volcengine/volcenginecc` 稳定。
- 本仓库的 API 边界是 Volcengine Cloud Control API。资源的 `TypeName`、属性名称、属性大小写和 Cloud Control schema 必须以服务端契约为准，不要根据 Terraform 资源名称自行推断 API 字段。
- 只实现 DPM 实际需要的资源和数据源。不要为了“完整覆盖”而增加未验证的 Cloud Control 资源，也不要引入通用抽象来解决单个资源的问题。
- `external-name` 表示真实 Volcengine 云资源 ID。不要把 Kubernetes UID、Terraform logical name 或人为生成的名称写入 `external-name`。

## 2. 上游同步与本地改动

- 优先保持官方 `terraform-provider-volcenginecc` 的目录结构、模块路径、package 名称和生成器约定。同步上游时，先确认上游提交涉及的生成器、schema 和 SDK 变更，再重新应用本仓库必要的 DPM 适配。
- DPM 适配应集中在少数明确位置，主要包括：
  - `internal/service/cloudcontrol/token.go`：Cloud Control 操作 Token；
  - `internal/generic/resource.go`：通用资源 Create/Update/Delete 调用；
  - `internal/provider/provider.go`：Provider 配置和运行时身份；
  - `internal/provider/generators/` 及 Cloud Control schema：生成资源的输入。
- 不要为了同步上游而覆盖其他开发者的未提交修改。同步前后都运行 `git diff --check`，并核对生成文件是否发生了预期之外的批量变化。
- 本仓库暂不通过修改 package 名称来区分 fork。需要区分 Volcengine fork 和官方版本时，使用 Git remote、版本号和变更记录，不要修改 import path。

## 3. Cloud Control 资源开发

- 资源代码通常是生成代码。修改资源前先检查对应的 Cloud Control schema、`internal/provider/schemas.go` 和 generator 配置；不要直接手改 `*_resource_gen.go`、`*_data_source_gen.go` 或生成测试文件。
- 资源变更的一般流程是：

  ```text
  Cloud Control schema / generator 输入
      -> make schemas
      -> make resources / make singular-data-sources / make plural-data-sources
      -> gofmt / goimports
      -> 单元测试和生成物检查
  ```

- 生成资源时必须确认：
  - Create、Read、Update、Delete handler 是否真实存在；
  - Identifier 的格式和 Read/Delete 所需的资源 ID 是否一致；
  - 异步操作是否返回 TaskID，以及最终 ProgressEvent 是否返回 Identifier；
  - 集合、Set、嵌套对象和敏感字段的语义是否与 Cloud Control schema 一致。
- 不要把 provider 配置字段、内部控制字段或运行时元数据加入资源的 `spec`/`TargetState`。这类字段必须留在 Terraform Provider 配置层。

## 4. 操作幂等、ClientToken 与失败重试

Cloud Control 对相同 ClientToken 回放原 task，**包括已经 `FAILED` 的 task**。因此 ClientToken 只能保护「一次结果未知的尝试」，不能把一个资源的某种操作绑死在一个 token 上。所有 Create / Update / Delete 都必须经过 `internal/service/cloudcontrol.Run`，不要在资源代码里直接调用 `CreateResourceWithContext` / `UpdateResourceWithContext` / `DeleteResourceWithContext`。

执行器约定（`internal/service/cloudcontrol/operation.go`）：

- 第一次提交总是使用稳定 token（`StableToken`），用于在超时或 provider 重启后接续仍在 `IN_PROGRESS` 的 task，而不是重复开操作。
- `IN_PROGRESS` / `PENDING` 只轮询 `GetTask`，不再提交。
- `FAILED` 分类（`retry.go`）：
  - NotFound：Delete 视为成功，Create/Update 返回 `tfresource.NotFoundError`；
  - 回放（`EventTime` 早于本次尝试开始，超出 `ReplaySkew`）：不论错误码，换 `AttemptToken` 再提交一次，拿到本次真实结论；
  - 可重试（`InvalidLock`、`ResourceConflict`、限流、`InternalError`、`ServiceUnavailable` 等，匹配 `ErrorCode` 和 `StatusMessage`）：换 `AttemptToken`，指数退避后重试，受 `MaxAttempts` 与操作 `Timeout` 约束；
  - 不可重试（权限、参数、配额、未知码）：立即返回，由调用方（Terraform / Crossplane）决定下一轮。下一轮的稳定 token 会回放这次 FAILED，被判定为回放后再真实提交一次，因此任何错误每个 reconcile 最多打一次真实 API，不会热循环。
- Create 的 `FAILED` 若带 `Identifier`，不得再提交（可能已留下部分资源），直接返回带 Identifier 的错误。
- 传输层错误（5xx、429、网络）用**同一个** token 重试；请求可能已被接受。
- 错误信息只含 type、identifier、task_id、request_id、error_code、status_message、event_time、attempts、class；不要拼接请求体。

稳定 token 的身份规则：

- Upjet/Crossplane 场景 Create：Upjet 将 Managed Resource 的 Kubernetes UID 作为 provider 配置 `crossplane_uid` 注入，Provider 使用 `create + TypeName + UID`（`CreateOperationToken`）。
- 直接 Terraform 场景 Create：没有 `crossplane_uid` 时使用每次唯一的随机 token，避免相同类型相同配置的两个资源碰撞；这种模式不承诺请求结果丢失后的跨重启幂等。
- Update：`UpdateOperationToken(TypeName, id, patchDocument)`；Delete：`DeleteOperationToken(TypeName, id)`。不要在这两条路径重新使用 Kubernetes UID。
- 不要仅使用 `ClusterId`、`Type` 等业务字段作为 Create token 身份。
- 不要把 `crossplane_uid` 写入 `desiredState`、Cloud Control `TargetState`、资源 CRD 或云资源属性。

改 token 算法、执行器或分类表时必须同步 `token_test.go` / `operation_test.go`，至少覆盖：相同 UID 同 token、不同 UID 不同 token、UID 为空随机 token；回放 FAILED → 新 token；`IN_PROGRESS` 只轮询；NotFound 不重提；不可重试一次即停；重试预算耗尽；Create 带 Identifier 不重提；传输错误同 token 重试。

相关实现：`internal/service/cloudcontrol/operation.go`、`retry.go`、`token.go`、`internal/generic/resource.go`、`internal/provider/provider.go`。

## 5. Provider 配置与 Upjet 集成

- Provider schema 中的内部字段必须是 Optional，且不能出现在任何资源 CRD 中。当前内部字段为 `crossplane_uid`。
- `provider-upjet-volcengine/internal/clients/volcengine.go` 只负责把 ProviderConfig 凭据和 Managed Resource UID 转换成 Terraform Provider 配置。不要将 UID 拼入 credentials secret、资源参数或 Cloud Control 属性。
- 凭据转换使用显式 allowlist，只传递 Provider 支持的配置键。新增配置键时必须同步：
  - Terraform Provider schema；
  - `configModel`；
  - Upjet credential conversion；
  - schema 生成结果；
  - 对应测试。
- Provider source 保持 `volcengine/volcenginecc`。Upjet 的本地开发可以通过 `go.mod replace` 指向本地 fork，但发布前必须确认版本、镜像和依赖路径指向预期版本。
- Provider 版本升级时，检查 Upjet 的 `terraform.ProviderRequirement.Version`、Terraform schema、生成 CRD 和 controller 是否仍然一致。

## 6. 生成物与命令

修改 Provider schema 或资源后，不要只修改源码而跳过生成物。

常用命令：

```bash
# 格式和基础验证
gofmt -w <changed-go-files>
go vet ./...
go test ./...
go build ./...

# 资源和数据源生成
make schemas
make resources
make singular-data-sources
make plural-data-sources

# 生成给 Upjet 使用的 Terraform provider schema，需要 terraform CLI
make schema
```

`make schema` 会在仓库根目录生成 `schema.json`。当 Terraform Provider schema 发生变化时，应将生成结果同步到 `provider-upjet-volcengine/config/schema.json`。同步后确认 `crossplane_uid` 只存在于 provider schema，不存在于 `package/crds` 或资源属性中。

生成代码、schema 和文档的差异应单独检查；不要提交与本次修改无关的全量生成漂移。

## 7. 测试要求

- 修改 Token、Provider 配置或通用资源逻辑时，至少运行对应 package 的 targeted tests，再运行 `go test ./...`。
- `go test ./...` 可能包含需要真实 Volcengine 凭据的 Cloud Control SDK 测试。遇到 `InvalidAuthorization`、网络或权限错误时，记录为环境/凭据问题，不要为了让测试通过而放宽认证逻辑。
- 接受测试需要显式设置 `TF_ACC=1`，并使用专用测试账号、区域和资源配额：

  ```bash
  TF_ACC=1 make testacc
  ```

- 不要在单元测试、日志或错误信息中输出 AccessKey、SecretKey、SessionToken、Proxy-Authorization、kubeconfig 或完整 credentials。
- 对异步操作测试应覆盖空响应、缺失 TaskID、失败 ProgressEvent 和缺失 Identifier 等边界；测试应验证错误上下文，而不是依赖完整响应 JSON 中可能包含的敏感字段。

## 8. 安全与日志

- 凭据字段必须标记为 Sensitive。新增凭据、代理认证或临时令牌字段时沿用该约定。
- 生产日志只记录资源类型、资源 ID、TaskID、RequestID、操作状态和错误码等必要元数据。不要记录完整 desired state、PatchDocument、Properties 或认证头。
- 不要把 secret、token、kubeconfig、ProviderConfig 内容或 Terraform state 提交到 Git。示例文件使用占位符。
- 错误信息应保留可定位问题的上下文，但避免拼接服务端完整响应；特别是不要把请求参数和认证信息直接放入 error message。

## 9. 发布前检查

发布或交给 Upjet 集成前，至少确认：

1. Go module path、Provider source 和 package 名称仍为 `volcengine`。
2. 必要的生成文件、`schema.json` 和版本信息已同步。
3. `crossplane_uid` 只作为内部 Provider 配置存在，未进入资源 CRD 或 Cloud Control TargetState。
4. Create/Update/Delete 的 Token 身份符合本文件约定。
5. `go vet ./...`、`go test ./...`、`go build ./...` 的结果已记录；真实 API 测试失败时已区分代码失败和环境失败。
6. `git diff --check` 通过，且 diff 中没有无关的生成物、凭据、调试日志或大范围格式化。
