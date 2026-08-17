resource "volcenginecc_transitrouter_transit_router_traffic_qos_marking_entry" "example" {
  transit_router_traffic_qos_marking_policy_id  = "tr-qos-marking-policy-xxxxxxxx"
  transit_router_traffic_qos_marking_entry_name = "ccapi-qos-marking-entry-full"
  priority                                      = 20
  protocol                                      = "tcp"
  source_cidr_block                             = "10.0.0.0/24"
  destination_cidr_block                        = "172.16.0.0/24"
  source_port_start                             = 1000
  source_port_end                               = 2000
  destination_port_start                        = 3000
  destination_port_end                          = 4000
  match_dscp                                    = 10
  remarking_dscp                                = 50
  description                                   = "traffic qos marking entry full fields"
}
