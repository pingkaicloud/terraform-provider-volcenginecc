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

resource "volcenginecc_vpc_security_group" "null_concrete" {
  vpc_id              = "vpc-rrco37ovjq4gv0x58zft8ul"
  security_group_name = "tf-multiset-null-concrete-e2e-sg"
  description         = "Temporary security group for null-to-concrete Multiset validation"
  project_name        = "default"
}

resource "volcenginecc_ecs_launch_template" "null_concrete" {
  launch_template_name         = "tf-multiset-null-concrete-e2e-template"
  launch_template_project_name = "default"

  launch_template_version = {
    description                   = "Temporary launch template for null-to-concrete validation"
    image_id                      = "image-aagd56zrvqjtdripcokn"
    instance_charge_type          = "PostPaid"
    instance_name                 = "tf-multiset-null-concrete-e2e"
    instance_type_id              = "ecs.g4il.large"
    key_pair_name                 = "MigrationKey-job-yecd7dromy38dfaxgxt8"
    project_name                  = "default"
    security_enhancement_strategy = "InActive"
    spot_strategy                 = "NoSpot"
    version_description           = "null-to-concrete validation"
    vpc_id                        = "vpc-rrco37ovjq4gv0x58zft8ul"
    zone_id                       = "cn-beijing-a"

    network_interfaces = [
      {
        subnet_id          = "subnet-rrwqhg3qzxfkv0x57g3edcq"
        security_group_ids = [volcenginecc_vpc_security_group.null_concrete.security_group_id]
      }
    ]
  }
}

resource "volcenginecc_ecs_launch_template_version" "null_concrete" {
  launch_template_id            = volcenginecc_ecs_launch_template.null_concrete.launch_template_id
  description                   = "Temporary launch template version for null-to-concrete validation"
  image_id                      = "image-aagd56zrvqjtdripcokn"
  instance_charge_type          = "PostPaid"
  instance_name                 = "tf-multiset-null-concrete-e2e"
  instance_type_id              = "ecs.g4il.large"
  key_pair_name                 = "MigrationKey-job-yecd7dromy38dfaxgxt8"
  project_name                  = "default"
  security_enhancement_strategy = "InActive"
  spot_strategy                 = "NoSpot"
  version_description           = "null-to-concrete validation"
  vpc_id                        = "vpc-rrco37ovjq4gv0x58zft8ul"
  zone_id                       = "cn-beijing-a"

  network_interfaces = [
    {
      subnet_id          = "subnet-rrwqhg3qzxfkv0x57g3edcq"
      security_group_ids = [volcenginecc_vpc_security_group.null_concrete.security_group_id]
    }
  ]

  volumes = [
    {
      delete_with_instance = true
      size                 = 40
      volume_type          = "ESSD_PL0"
    }
  ]
}

resource "volcenginecc_autoscaling_scaling_group" "null_concrete" {
  scaling_group_name      = "tf-multiset-null-concrete-e2e-group"
  subnet_ids              = ["subnet-rrwqhg3qzxfkv0x57g3edcq"]
  min_instance_number     = 0
  max_instance_number     = 0
  desire_instance_number  = -1
  health_check_type       = "NONE"
  project_name            = "default"
  launch_template_id      = volcenginecc_ecs_launch_template.null_concrete.launch_template_id
  launch_template_version = "Latest"
  depends_on              = [volcenginecc_ecs_launch_template_version.null_concrete]
}

resource "volcenginecc_autoscaling_scaling_configuration" "null_concrete" {
  scaling_group_id              = volcenginecc_autoscaling_scaling_group.null_concrete.scaling_group_id
  scaling_configuration_name    = "tf-multiset-null-concrete-e2e-config"
  image_id                      = "image-aagd56zrvqjtdripcokn"
  instance_name                 = "tf-multiset-null-concrete-e2e"
  key_pair_name                 = "MigrationKey-job-yecd7dromy38dfaxgxt8"
  security_group_ids            = [volcenginecc_vpc_security_group.null_concrete.security_group_id]
  zone_id                       = "cn-beijing-a"
  project_name                  = "default"
  security_enhancement_strategy = "InActive"
  spot_strategy                 = "SpotWithPriceLimit"

  instance_type_overrides = [
    {
      instance_type = "ecs.g4il.large"
      price_limit   = 1
    }
  ]

  volumes = [
    {
      size        = 40
      volume_type = "ESSD_PL0"
    },
    {
      size        = 50
      volume_type = "ESSD_PL0"
    }
  ]
}

output "scaling_configuration_id" {
  value = volcenginecc_autoscaling_scaling_configuration.null_concrete.scaling_configuration_id
}
