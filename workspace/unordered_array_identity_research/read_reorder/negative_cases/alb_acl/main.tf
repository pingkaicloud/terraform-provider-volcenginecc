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

resource "volcenginecc_alb_acl" "identity" {
  acl_name     = "tf-identity-reorder-acl"
  description  = "Temporary ACL for identity reorder validation"
  project_name = "default"

  acl_entries = [
    {
      entry       = "10.251.0.0/25"
      description = "identity-entry-a"
    },
    {
      entry       = "10.251.0.128/25"
      description = "identity-entry-b"
    }
  ]
}

output "acl_id" {
  value = volcenginecc_alb_acl.identity.acl_id
}
