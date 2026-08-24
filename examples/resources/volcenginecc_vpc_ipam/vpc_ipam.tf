resource "volcenginecc_vpc_ipam" "example" {
  operating_regions = [
    "cn-beijing",
  ]
  project_name = "default"
  ipam_name    = "tf-ipam-full"
  description  = "created by terraform full case"
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
