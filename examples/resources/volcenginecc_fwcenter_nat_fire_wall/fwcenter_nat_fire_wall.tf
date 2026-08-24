resource "volcenginecc_fwcenter_nat_fire_wall" "example" {
  nat_gateway_id    = "ngw-xxxxxx"
  nat_firewall_name = "example"
  firewall_cidr     = "192.168.208.0/28"
  bandwidth         = 100
  project_name      = "default"
  cloud_firewall_id = "CloudFirewallxxxxxxxx"
}
