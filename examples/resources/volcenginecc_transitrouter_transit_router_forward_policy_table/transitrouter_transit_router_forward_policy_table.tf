resource "volcenginecc_transitrouter_transit_router_forward_policy_table" "example" {
  transit_router_id                        = "tr-xxxxxx"
  transit_router_forward_policy_table_name = "ccapi-forward-policy-table-full"
  description                              = "transit router forward policy table full fields"
  transit_router_attachment_ids            = ["attachment_id-xxxx"]
}
