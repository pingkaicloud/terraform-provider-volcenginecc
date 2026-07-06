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

resource "volcenginecc_vpc_security_group" "case" {
  vpc_id              = "vpc-rrco37ovjq4gv0x58zft8ul"
  security_group_name = "tf-set-stable-null-unknown-sg-20260702"
  description         = "SetNestedAttribute stable null to unknown E2E"
  project_name        = "default"
}

resource "volcenginecc_vpc_eni" "case" {
  network_interface_name = "tf-set-stable-null-unknown-eni-20260702"
  description            = "SetNestedAttribute stable null to unknown E2E"
  subnet_id              = "subnet-rrwqhg3qzxfkv0x57g3edcq"
  security_group_ids     = [volcenginecc_vpc_security_group.case.security_group_id]
  project_name           = "default"

  private_ip_sets = [
    {
      private_ip_address = "192.168.0.221"
    }
  ]

  tags = []
}

output "network_interface_id" {
  value = volcenginecc_vpc_eni.case.network_interface_id
}
