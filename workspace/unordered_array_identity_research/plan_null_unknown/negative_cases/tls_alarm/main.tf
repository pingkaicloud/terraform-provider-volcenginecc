terraform {
  required_version = ">= 1.0.7"

  required_providers {
    volcenginecc = {
      source = "volcengine/volcenginecc"
    }
  }
}

provider "volcenginecc" {
  region    = "cn-beijing"
  profile   = "default"
  file_path = "/Users/bytedance/.volcengine/config.json"
}

data "volcenginecc_tls_alarm_notify_groups" "available" {}

resource "volcenginecc_tls_project" "case" {
  project_name     = "tf-set-stable-null-unknown-20260702"
  description      = "SetNestedAttribute stable null to unknown E2E"
  iam_project_name = "default"
}

resource "volcenginecc_tls_topic" "case" {
  project_id    = volcenginecc_tls_project.case.project_id
  topic_name    = "tf-set-stable-null-unknown-topic-20260702"
  description   = "SetNestedAttribute stable null to unknown E2E"
  ttl           = 1
  shard_count   = 1
  log_public_ip = false
}

resource "volcenginecc_tls_alarm" "case" {
  alarm_name     = "tf-set-stable-null-unknown-20260702"
  project_id     = volcenginecc_tls_project.case.project_id
  status         = false
  trigger_period = 1
  alarm_period   = 10
  alarm_notify_groups = [
    {
      alarm_notify_group_id = tolist(data.volcenginecc_tls_alarm_notify_groups.available.ids)[0]
    }
  ]

  query_requests = [
    {
      query             = "*"
      start_time_offset = -10
      end_time_offset   = 0
      topic_id          = volcenginecc_tls_topic.case.topic_id
    }
  ]

  request_cycle = {
    type = "Period"
    time = 1
  }

  condition = "$1.__count__ > 0"
  severity  = "notice"
}

output "alarm_id" {
  value = volcenginecc_tls_alarm.case.alarm_id
}
