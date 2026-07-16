resource "volcenginecc_escloud_ip_allow_list" "Example" {
  instance_id = "o-008wv7krnmw4"
  type        = "PRIVATE_ES"
  allow_list  = ["10.0.0.0/16", "192.168.1.1"]
  groups = [
    {
      name       = "group1"
      allow_list = ["10.0.0.0/16", "192.168.1.2"]
    },
    {
      name       = "group2"
      allow_list = ["192.168.2.1", "127.0.0.1"]
    }
  ]
}