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

resource "volcenginecc_vpc_vpc" "identity" {
  cidr_block   = "10.249.0.0/24"
  vpc_name     = "tf-unordered-identity-e1-0630-vpc"
  description  = "Temporary VPC for unordered identity validation"
  project_name = "default"
}

resource "volcenginecc_vpc_subnet" "identity" {
  vpc_id      = volcenginecc_vpc_vpc.identity.vpc_id
  zone_id     = "cn-beijing-a"
  subnet_name = "tf-unordered-identity-e1-0630-subnet"
  description = "Temporary subnet for unordered identity validation"
  cidr_block  = "10.249.0.0/25"
}

resource "volcenginecc_vpc_security_group" "identity" {
  vpc_id              = volcenginecc_vpc_vpc.identity.vpc_id
  security_group_name = "tf-unordered-identity-e1-0630-sg"
  description         = "Temporary security group for unordered identity validation"
  project_name        = "default"
}

resource "volcenginecc_ecs_launch_template" "identity" {
  launch_template_name         = "tf-unordered-identity-e1-0630-template"
  launch_template_project_name = "default"

  launch_template_version = {
    description                   = "Temporary launch template for unordered identity validation"
    image_id                      = "image-aagd56zrvqjtdripcokn"
    instance_charge_type          = "PostPaid"
    instance_name                 = "tf-unordered-identity-e2e-instance"
    instance_type_id              = "ecs.g4il.large"
    key_pair_name                 = "MigrationKey-job-yecd7dromy38dfaxgxt8"
    project_name                  = "default"
    security_enhancement_strategy = "InActive"
    spot_strategy                 = "NoSpot"
    version_description           = "identity validation"
    vpc_id                        = volcenginecc_vpc_vpc.identity.vpc_id
    zone_id                       = "cn-beijing-a"

    network_interfaces = [
      {
        subnet_id          = volcenginecc_vpc_subnet.identity.subnet_id
        security_group_ids = [volcenginecc_vpc_security_group.identity.security_group_id]
      }
    ]
  }
}

resource "volcenginecc_ecs_launch_template_version" "identity" {
  launch_template_id            = volcenginecc_ecs_launch_template.identity.launch_template_id
  description                   = "Temporary launch template version for identity validation"
  image_id                      = "image-aagd56zrvqjtdripcokn"
  instance_charge_type          = "PostPaid"
  instance_name                 = "tf-unordered-identity-e2e-instance"
  instance_type_id              = "ecs.g4il.large"
  key_pair_name                 = "MigrationKey-job-yecd7dromy38dfaxgxt8"
  project_name                  = "default"
  security_enhancement_strategy = "InActive"
  spot_strategy                 = "NoSpot"
  version_description           = "identity validation"
  vpc_id                        = volcenginecc_vpc_vpc.identity.vpc_id
  zone_id                       = "cn-beijing-a"

  network_interfaces = [
    {
      subnet_id          = volcenginecc_vpc_subnet.identity.subnet_id
      security_group_ids = [volcenginecc_vpc_security_group.identity.security_group_id]
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

resource "volcenginecc_autoscaling_scaling_group" "identity" {
  scaling_group_name     = "tf-unordered-identity-e1-0630-group"
  subnet_ids             = [volcenginecc_vpc_subnet.identity.subnet_id]
  min_instance_number    = 0
  max_instance_number    = 0
  desire_instance_number = -1
  health_check_type      = "NONE"
  project_name           = "default"
  depends_on             = [volcenginecc_ecs_launch_template_version.identity]
}

resource "volcenginecc_autoscaling_scaling_configuration" "identity" {
  scaling_group_id              = volcenginecc_autoscaling_scaling_group.identity.scaling_group_id
  scaling_configuration_name    = "tf-unordered-identity-e1-0630-config"
  image_id                      = "image-aagd56zrvqjtdripcokn"
  instance_name                 = "tf-unordered-identity-e2e-instance"
  key_pair_name                 = "MigrationKey-job-yecd7dromy38dfaxgxt8"
  security_group_ids            = [volcenginecc_vpc_security_group.identity.security_group_id]
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
      delete_with_instance = true
      size                 = 40
      volume_type          = "ESSD_PL0"
    },
    {
      delete_with_instance = true
      size                 = 60
      volume_type          = "ESSD_PL0"
    }
  ]
}

output "scaling_configuration_id" {
  value = volcenginecc_autoscaling_scaling_configuration.identity.scaling_configuration_id
}
