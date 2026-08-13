resource "volcenginecc_vpc_ip_pool_cidr_block" "example" {
  ip_address_pool_id = "ippool-xxxxxxxxxxxxxxxxxxxx"
  cidr_block         = "198.51.100.0/28"
}
