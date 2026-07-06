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
  security_group_name = "tf-identity-read-reorder-sg"
  description         = "Temporary security group for identity read reorder validation"
  project_name        = "default"
}

resource "volcenginecc_ecs_instance" "read_reorder" {
  instance_name        = "tf-identity-read-reorder-instance"
  hostname             = "tf-identity-read-reorder-instance"
  description          = "Temporary ECS instance for identity read reorder validation"
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
    security_group_ids = [volcenginecc_vpc_security_group.read_reorder.security_group_id]
  }

  secondary_network_interfaces = [
    {
      subnet_id          = "subnet-rrwqhg3qzxfkv0x57g3edcq"
      security_group_ids = [volcenginecc_vpc_security_group.read_reorder.security_group_id]
    },
    {
      subnet_id          = "subnet-rrwqhg3qzxfkv0x57g3edcq"
      security_group_ids = [volcenginecc_vpc_security_group.read_reorder.security_group_id]
    }
  ]

  system_volume = {
    size                 = 50
    delete_with_instance = true
    volume_type          = "ESSD_PL0"
  }
}

output "instance_id" {
  value = volcenginecc_ecs_instance.read_reorder.instance_id
}
