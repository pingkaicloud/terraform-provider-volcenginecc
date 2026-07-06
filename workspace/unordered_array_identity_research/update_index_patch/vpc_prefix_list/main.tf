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

resource "volcenginecc_vpc_prefix_list" "identity" {
  prefix_list_name = "tf-identity-stage-f-0630"
  description      = "Temporary prefix list for identity reorder validation"
  ip_version       = "IPv4"
  max_entries      = 10
  project_name     = "default"

  prefix_list_entries = [
    {
      cidr        = "10.252.0.0/25"
      description = "identity-entry-a"
    },
    {
      cidr        = "10.252.0.128/25"
      description = "identity-entry-b-updated"
    }
  ]
}

output "prefix_list_id" {
  value = volcenginecc_vpc_prefix_list.identity.prefix_list_id
}
