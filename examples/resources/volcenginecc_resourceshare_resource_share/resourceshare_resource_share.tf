resource "volcenginecc_resourceshare_resource_share" "example" {
  resource_share_name = "cc-test-example"
  allow_share_type    = "ANY"
  resource_trns       = "trn:vpc:cn-beijing:210000****:subnet/subnet-example"
  principals          = "210000****,210001****"

  tags = [
    {
      key   = "key1"
      value = "value1"
    },
    {
      key   = "key2"
      value = "value2"
    }
  ]
}
