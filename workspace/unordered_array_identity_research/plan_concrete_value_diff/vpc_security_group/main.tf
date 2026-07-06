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

resource "volcenginecc_vpc_security_group" "null_unknown" {
  vpc_id              = "vpc-rrco37ovjq4gv0x58zft8ul"
  security_group_name = "tf-set-null-unknown-isolated-20260628"
  description         = "Isolated SetNestedAttribute null to unknown validation"
  project_name        = "default"

  ingress_permissions = [
    {
      description = "set-null-unknown-ssh"
      direction   = ""
      policy      = "accept"
      port_start  = 22
      port_end    = 22
      priority    = 20
      protocol    = "tcp"
      cidr_ip     = "10.250.0.0/25"
    }
  ]

  tags = [
    {
      key   = "case"
      value = "set-null-unknown"
    }
  ]
}

output "security_group_id" {
  value = volcenginecc_vpc_security_group.null_unknown.security_group_id
}
