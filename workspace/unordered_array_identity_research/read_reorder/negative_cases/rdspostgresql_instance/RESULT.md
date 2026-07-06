# RDSPostgreSQL NodeInfo Read 反序候选验证结果

> 日期：2026-06-29  
> 分支：`terraform-identity-validation`  
> 目录：`workspace/unordered_array_identity_research/read_reorder/negative_cases/rdspostgresql_instance`

## 目标

验证 `Volcengine::RDSPostgreSQL::Instance.NodeInfo` 是否满足阶段 E 真实云验收条件：

- 生成形态为 `ListNestedAttribute + generic.Multiset()`；
- 元素存在用户可知、服务端稳定读回的 identity 候选；
- 真实 `UpdateResource` 反序提交后，`GetResource` 能保留不同于 config/prior 的顺序；
- 若服务端保留反序，再验证修复后 Read 能按 identity 对齐并收敛。

## 候选条件

`NodeInfo` 的静态条件满足：

- Cloud Control schema 为 `insertionOrder=false`、`uniqueItems=false`；
- 生成 Terraform schema 为 required `ListNestedAttribute + generic.Multiset()`；
- 元素内 `ZoneId`、`NodeType`、`NodeSpec` 均为 required；
- 本轮使用 `ZoneId + NodeType + NodeSpec` 作为 identity 候选。

## 执行过程

1. 创建临时 `volcenginecc_rdspostgresql_instance.read_reorder`：
   - `instance_id = postgres-bba93d7650fb`
   - `node_info[0] = Primary`
   - `node_info[1] = Secondary`
2. 创建耗时约 7 分 20 秒。
3. 基线 `terraform plan -refresh=true -detailed-exitcode` 为 `No changes`。
4. 执行 `reorder_rdspostgresql_node_info.go`：
   - 先读 `GetResource.NodeInfo`；
   - 将两个元素反序；
   - 通过 `UpdateResource` replace `/NodeInfo`；
   - 等待任务完成后再次 `GetResource`。

## 关键证据

`logs/phase-e-pg-reorder-remote.log`：

```text
before:
0: zone=cn-beijing-a type=Primary spec=rds.postgres.1c2g node_id=postgres-bba93d7650fb status=Running
1: zone=cn-beijing-a type=Secondary spec=rds.postgres.1c2g node_id=postgres-bba93d7650fb-bqut status=Running
after:
0: zone=cn-beijing-a type=Primary spec=rds.postgres.1c2g node_id=postgres-bba93d7650fb status=Running
1: zone=cn-beijing-a type=Secondary spec=rds.postgres.1c2g node_id=postgres-bba93d7650fb-bqut status=Running
```

说明真实 `UpdateResource` 反序提交成功，但后续 `GetResource` 仍返回服务端规范顺序：
`Primary, Secondary`。

## 结论

`RDSPostgreSQL.Instance.NodeInfo` 不能作为阶段 E 的合格真实验收资源。

原因不是 identity 候选缺失，而是服务端不保留反序；它无法制造
“远端 GetResource 稳定返回不同于 config/prior 的顺序”这一验收条件。

## VEDBM Nodes 静态排除

同步检查 `Volcengine::VEDBM::Instance.Nodes`：

- 生成形态确实是 `ListNestedAttribute + generic.Multiset()`；
- 但元素内 `NodeId`、`NodeSpec`、`ZoneId`、`Memory`、`vCPU` 等主要字段均为
  Computed；
- `NodeType` 是 `Optional + Computed`，不足以在多节点场景中形成用户可知且唯一的
  identity；
- 因此不满足阶段 E 目标中的“元素里有用户可知、服务端稳定读回的 identity 字段”。

本轮未创建 VEDBM 实例。

## 清理

已执行：

```sh
TF_CLI_CONFIG_FILE=/private/tmp/terraform-provider-volcenginecc-dev.tfrc terraform destroy -auto-approve -input=false
TF_CLI_CONFIG_FILE=/private/tmp/terraform-provider-volcenginecc-dev.tfrc terraform state list
```

`logs/phase-e-pg-final-state-list.log` 为空，确认本 case 创建的 Terraform 资源已清理。
