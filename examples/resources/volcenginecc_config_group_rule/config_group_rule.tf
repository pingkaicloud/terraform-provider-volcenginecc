resource "volcenginecc_config_group_rule" "example" {
  account_group_id = "00000000-0000-0000-0000-000000000000"
  rule_template_id = "00000000-0000-0000-0000-000000000000"
  rule_name        = "ccapi-config-grouprule-full"
  description      = "ccapi config group rule full"
  triggers = [{
    trigger_type                = "Periodic"
    maximum_execution_frequency = "TwentyFourHours"
  }]
  input_parameters = "{}"
  scope = {
    resource_types = {
      include = ["Volcengine::Redis::AllowList"]
    }
    resource_ids = {
      include = ["example-resource-id"]
    }
    regions = {
      include = ["cn-beijing"]
    }
    projects = {
      include = ["default"]
    }
    tags = {
      include = [{
        key   = "k1"
        value = "v1"
      }]
    }
  }
  risk_level = "Medium"
}
