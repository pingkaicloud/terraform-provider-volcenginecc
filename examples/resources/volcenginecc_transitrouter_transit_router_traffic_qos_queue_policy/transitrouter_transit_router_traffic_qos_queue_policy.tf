resource "volcenginecc_transitrouter_transit_router_traffic_qos_queue_policy" "example" {
  transit_router_id                            = "tr-xxxxxx"
  transit_router_traffic_qos_queue_policy_name = "ccapi-qos-policy-full"
  description                                  = "traffic qos queue policy full fields"
}
