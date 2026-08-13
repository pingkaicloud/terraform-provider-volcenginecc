resource "volcenginecc_vpc_ip_pool" "example" {
  isp          = "BGP"
  cidr_mask    = 28
  description  = "ccapi ip pool with cidr mask"
  name         = "ccapi-ippool-cidr-mask"
  project_name = "default"
  tags = [{
    key   = "k1"
    value = "v1"
  }]
}
