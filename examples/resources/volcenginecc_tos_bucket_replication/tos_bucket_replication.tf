resource "volcenginecc_tos_bucket_replication" "Example" {
  role   = "ServiceRoleforReplicationAccessTOS"
  bucket = "ccapi-test-16"
  rules = [{
    id         = "rule-002"
    prefix_set = ["prefix_1", "prefix_2"]
    status     = "Enabled"
    destination = {
      bucket                          = "ccapi-test-7"
      location                        = "cn-beijing"
      storage_class                   = "STANDARD"
      storage_class_inherit_directive = "SOURCE_OBJECT"
    }
    historical_object_replication = "Enabled"
    access_control_translation = {
      owner = "BucketOwnerEntrusted"
    }
  }]

}