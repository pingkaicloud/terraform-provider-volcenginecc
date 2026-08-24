resource "volcenginecc_kafka_acl" "example" {
  instance_id   = "kafka-xxxxxx"
  user_name     = "*"
  ip            = "*"
  resource_type = "Topic"
  pattern_type  = "Literal"
  resource      = "*"
  access_policy = "Write"
}
