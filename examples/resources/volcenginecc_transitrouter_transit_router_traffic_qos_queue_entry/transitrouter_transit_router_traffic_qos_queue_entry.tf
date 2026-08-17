resource "volcenginecc_transitrouter_transit_router_traffic_qos_queue_entry" "example" {
  transit_router_traffic_qos_queue_policy_id  = "tr-qos-queue-policy-xxxxxxxx"
  transit_router_traffic_qos_queue_entry_name = "ccapi-qos-entry-full"
  description                                 = "traffic qos queue entry full fields"
  bandwidth_percent                           = 30
  dscps                                       = [20, 21]
  priority                                    = "Normal"
}
