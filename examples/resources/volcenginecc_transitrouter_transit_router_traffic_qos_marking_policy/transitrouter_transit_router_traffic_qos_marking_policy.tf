resource "volcenginecc_transitrouter_transit_router_traffic_qos_marking_policy" "example" {
  transit_router_id                              = "tr-xxxxxxxx"
  transit_router_traffic_qos_marking_policy_name = "ccapi-qos-marking-policy-full"
  description                                    = "traffic qos marking policy full fields"
}
