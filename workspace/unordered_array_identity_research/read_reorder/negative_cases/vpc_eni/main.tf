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

resource "volcenginecc_vpc_security_group" "read_reorder" {
  vpc_id              = "vpc-rrco37ovjq4gv0x58zft8ul"
  security_group_name = "tf-eni-read-reorder-sg"
  description         = "Temporary security group for ENI read reorder validation"
  project_name        = "default"
}

resource "volcenginecc_vpc_eni" "read_reorder" {
  network_interface_name = "tf-eni-read-reorder"
  description            = "Temporary ENI for identity read reorder validation"
  subnet_id              = "subnet-rrwqhg3qzxfkv0x57g3edcq"
  security_group_ids     = [volcenginecc_vpc_security_group.read_reorder.security_group_id]
  project_name           = "default"

  private_ip_sets = [
    {
      private_ip_address = "192.168.0.223"
    },
    {
      private_ip_address = "192.168.0.224"
    }
  ]
}

output "network_interface_id" {
  value = volcenginecc_vpc_eni.read_reorder.network_interface_id
}

