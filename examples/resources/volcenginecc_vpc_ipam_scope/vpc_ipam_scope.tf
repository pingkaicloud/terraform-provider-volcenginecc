resource "volcenginecc_vpc_ipam_scope" "example" {
  ipam_id         = "ipam-xxxxxx"
  ipam_scope_name = "tf-ipam-scope-full"
  ipam_scope_type = "private"
  description     = "created by terraform full case"
  project_name    = "default"

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
