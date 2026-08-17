resource "volcenginecc_config_configuration_recorder" "example" {
  recorder_type              = "SingleAccount"
  include_all_resource_types = false
  include_resource_types     = ["Volcengine::ALB::ACL"]
}
