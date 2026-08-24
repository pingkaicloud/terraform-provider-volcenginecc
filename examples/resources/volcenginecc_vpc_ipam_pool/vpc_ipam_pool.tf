resource "volcenginecc_vpc_ipam_pool" "example" {
  ipam_scope_id  = "ipam-scope-xxxxxx"
  ip_version     = "IPv4"
  pool_region_id = "cn-beijing"
  project_name   = "default"

  ipam_pool_name = "tf-ipam-pool-full"
  description    = "created by terraform full case"
  auto_import    = true

  allocation_default_cidr_mask = 24
  allocation_min_cidr_mask     = 16
  allocation_max_cidr_mask     = 28

  tags = [
    {
      key   = "case"
      value = "tf-c02"
    },
    {
      key   = "env"
      value = "terraform"
    }
  ]
}
