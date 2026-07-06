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

resource "volcenginecc_vpc_security_group" "plan" {
  vpc_id              = "vpc-rrco37ovjq4gv0x58zft8ul"
  security_group_name = "tf-multiset-plan-e2e-sg"
  description         = "Temporary security group for Multiset plan validation"
  project_name        = "default"
}

resource "volcenginecc_ecs_instance" "plan" {
  instance_name        = "tf-multiset-plan-e2e-instance"
  hostname             = "tf-multiset-plan-e2e-instance"
  description          = "Temporary ECS instance for Multiset plan validation"
  project_name         = "default"
  zone_id              = "cn-beijing-a"
  instance_type        = "ecs.g4il.large"
  instance_charge_type = "PostPaid"
  spot_strategy        = "NoSpot"
  deletion_protection  = false

  image = {
    image_id = "image-aagd56zrvqjtdripcokn"
  }

  key_pair = {
    key_pair_name = "MigrationKey-job-yecd7dromy38dfaxgxt8"
  }

  primary_network_interface = {
    subnet_id          = "subnet-rrwqhg3qzxfkv0x57g3edcq"
    security_group_ids = [volcenginecc_vpc_security_group.plan.security_group_id]
  }

  secondary_network_interfaces = [
    {
      subnet_id          = "subnet-rrwqhg3qzxfkv0x57g3edcq"
      security_group_ids = [volcenginecc_vpc_security_group.plan.security_group_id]
    }
  ]

  system_volume = {
    size                 = 50
    delete_with_instance = true
    volume_type          = "ESSD_PL0"
  }
}

output "instance_id" {
  value = volcenginecc_ecs_instance.plan.instance_id
}
