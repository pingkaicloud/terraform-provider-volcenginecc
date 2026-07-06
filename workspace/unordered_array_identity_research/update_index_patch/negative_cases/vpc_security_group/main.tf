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

resource "volcenginecc_vpc_vpc" "set_identity" {
  cidr_block   = "10.250.0.0/24"
  vpc_name     = "tf-unordered-set-e2e-vpc"
  description  = "Temporary VPC for SetNestedAttribute validation"
  project_name = "default"
}

resource "volcenginecc_vpc_security_group" "set_identity" {
  vpc_id              = volcenginecc_vpc_vpc.set_identity.vpc_id
  security_group_name = "tf-unordered-set-e2e-sg"
  description         = "Temporary security group for SetNestedAttribute validation"
  project_name        = "default"

  ingress_permissions = [
    {
      description     = "set-rule-ssh"
      policy          = "accept"
      port_start      = 22
      port_end        = 22
      priority        = 20
      protocol        = "tcp"
      cidr_ip         = "10.250.0.0/25"
      prefix_list_id  = ""
      source_group_id = ""
    },
    {
      description     = "set-rule-http"
      policy          = "accept"
      port_start      = 80
      port_end        = 80
      priority        = 30
      protocol        = "tcp"
      cidr_ip         = "10.250.0.128/25"
      prefix_list_id  = ""
      source_group_id = ""
    }
  ]
}

output "security_group_id" {
  value = volcenginecc_vpc_security_group.set_identity.security_group_id
}
