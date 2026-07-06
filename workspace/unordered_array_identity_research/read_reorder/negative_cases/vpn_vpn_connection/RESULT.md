# VPN TunnelOptions Read 反序候选验证结果

> 日期：2026-06-29  
> 分支：`terraform-identity-validation`  
> 目录：`workspace/unordered_array_identity_research/read_reorder/negative_cases/vpn_vpn_connection`

## 目标

验证 `Volcengine::VPN::VPNConnection.TunnelOptions` 是否满足阶段 E 真实云验收条件：

- 生成形态为 `ListNestedAttribute + generic.Multiset()`；
- 元素存在用户可知、服务端稳定读回的 identity 候选；
- 真实 `UpdateResource` 反序提交后，`GetResource` 能保留不同于 config/prior 的顺序；
- 若服务端保留反序，再验证修复后 Read 能按 identity 对齐并收敛。

## 候选条件

`TunnelOptions` 的静态条件基本满足：

- Cloud Control schema 为 `insertionOrder=false`、`uniqueItems=false`；
- 生成 Terraform schema 为 `ListNestedAttribute + generic.Multiset()`；
- 本轮真实配置包含两个元素：
  - `role = "primary"`，`customer_gateway_id = cgw-btiig7a3smbk5h0b2uvv4fis`；
  - `role = "secondary"`，`customer_gateway_id = cgw-btiig994woao5h0b2ui051r6`；
- `Role + CustomerGatewayId` 可以作为本 case 的用户可知 identity 候选。

## 执行过程

1. 初始尝试新建 secondary subnet `192.168.0.128/25`，Cloud Control 返回
   `InvalidSubnetCidr.Conflict`，说明该 VPC 已有冲突网段。
2. 通过 `ListResource(Volcengine::VPC::Subnet)` 查询同一 VPC 下已有 subnet，改为复用：
   `subnet-btnzu3hrc0005h0b2tluy7jg`。
3. `terraform apply` 成功创建：
   - `volcenginecc_vpn_customer_gateway.primary`
   - `volcenginecc_vpn_customer_gateway.secondary`
   - `volcenginecc_vpn_vpn_gateway.read_reorder`
   - `volcenginecc_vpn_vpn_connection.read_reorder`
4. 创建的 VPNConnection：
   `vgc-btixrckwge0w5h0b2tdz9uh2`。
5. 基线 `terraform plan -refresh=true -detailed-exitcode` 仍有既有
   `tunnel_bgp_info.enable_bgp` 的 `null -> unknown` 噪声，退出码为 2。
6. 执行 `reorder_vpn_tunnel_options.go`：
   - 先读 `GetResource.TunnelOptions`；
   - 将两个元素反序；
   - 通过 `UpdateResource` replace `/TunnelOptions`；
   - 等待任务完成后再次 `GetResource`。

## 关键证据

`logs/phase-e-vpn-reorder-remote.log`：

```text
before:
0: role=primary customer_gateway_id=cgw-btiig7a3smbk5h0b2uvv4fis tunnel_id=tunnel-btixrejxkg005h0b2timtcjs connect_status=ike_sa_negotiation_failed
1: role=secondary customer_gateway_id=cgw-btiig994woao5h0b2ui051r6 tunnel_id=tunnel-btixrgiyohz45h0b2tj77y6z connect_status=ike_sa_negotiation_failed
after:
0: role=primary customer_gateway_id=cgw-btiig7a3smbk5h0b2uvv4fis tunnel_id=tunnel-btixrejxkg005h0b2timtcjs connect_status=ike_sa_negotiation_failed
1: role=secondary customer_gateway_id=cgw-btiig994woao5h0b2ui051r6 tunnel_id=tunnel-btixrgiyohz45h0b2tj77y6z connect_status=ike_sa_negotiation_failed
```

说明真实 `UpdateResource` 反序提交成功，但后续 `GetResource` 仍返回服务端规范顺序：
`primary, secondary`。

## 结论

`VPNConnection.TunnelOptions` 不能作为阶段 E 的合格真实验收资源。

原因不是 identity 不可用，而是服务端不保留反序；它无法制造
“远端 GetResource 稳定返回不同于 config/prior 的顺序”这一验收条件。

## 其他候选环境检查

本轮只做轻量 `ListResource` 检查，未创建额外资源：

- `Volcengine::VEPFS::Instance`：当前账号/区域列表为空；
- `Volcengine::RDSPostgreSQL::Instance`：当前账号/区域列表为空；
- `Volcengine::VEDBM::Instance`：当前账号/区域列表为空。

对应日志文件均为空：

- `logs/phase-e-vepfs-list-instances.log`
- `logs/phase-e-rdspostgresql-list-instances.log`
- `logs/phase-e-vedbm-list-instances.log`

## 清理

已执行：

```sh
TF_CLI_CONFIG_FILE=/private/tmp/terraform-provider-volcenginecc-dev.tfrc terraform destroy -auto-approve -input=false
TF_CLI_CONFIG_FILE=/private/tmp/terraform-provider-volcenginecc-dev.tfrc terraform state list
```

`logs/phase-e-vpn-final-state-list.log` 为空，确认本 case 创建的 Terraform 资源已清理。
