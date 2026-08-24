resource "volcenginecc_transitrouter_transit_router_multicast_domain" "example" {
  transit_router_id                    = "tr-xxxxxx"
  transit_router_multicast_domain_name = "ccapi-multicast-domain-full"
  description                          = "transit router multicast domain full fields"
  tags = [
    {
      key   = "usage"
      value = "ccapi"
    },
    {
      key   = "business"
      value = "transit-router"
    }
  ]
}
