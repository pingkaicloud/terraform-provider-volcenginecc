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
  profile   = "volc-eps-ccapi-test"
  file_path = "/Users/bytedance/.volcengine/config.json"
}

locals {
  zone_id = "cn-beijing-a"
}

resource "volcenginecc_vpc_vpc" "vepfs" {
  cidr_block   = "10.251.0.0/24"
  vpc_name     = "tf-vepfs-read-reorder-vpc-0701"
  description  = "Temporary VPC for VEPFS identity validation"
  project_name = "default"
}

resource "volcenginecc_vpc_subnet" "vepfs" {
  vpc_id      = volcenginecc_vpc_vpc.vepfs.vpc_id
  zone_id     = local.zone_id
  subnet_name = "tf-vepfs-read-reorder-subnet-0701"
  description = "Temporary subnet for VEPFS identity validation"
  cidr_block  = "10.251.0.0/25"
}

resource "volcenginecc_vepfs_instance" "first" {
  file_system_name = "tf-vepfs-read-reorder-a"
  zone_id          = local.zone_id
  charge_type      = "PayAsYouGo"
  file_system_type = "VePFS"
  store_type       = "Advance_100"
  protocol_type    = "VePFS"
  project_name     = "default"
  capacity         = 6
  vpc_id           = volcenginecc_vpc_vpc.vepfs.vpc_id
  subnet_id        = volcenginecc_vpc_subnet.vepfs.subnet_id
  enable_restripe  = true
}

resource "volcenginecc_vepfs_instance" "second" {
  file_system_name = "tf-vepfs-read-reorder-b"
  zone_id          = local.zone_id
  charge_type      = "PayAsYouGo"
  file_system_type = "VePFS"
  store_type       = "Advance_100"
  protocol_type    = "VePFS"
  project_name     = "default"
  capacity         = 6
  vpc_id           = volcenginecc_vpc_vpc.vepfs.vpc_id
  subnet_id        = volcenginecc_vpc_subnet.vepfs.subnet_id
  enable_restripe  = true
}

resource "volcenginecc_vepfs_mount_service" "read_reorder" {
  mount_service_name = "tf-vepfs-read-reorder-ms"
  project            = "default"
  node_type          = "ecs.g4i.large"
  subnet_id          = volcenginecc_vpc_subnet.vepfs.subnet_id
  vpc_id             = volcenginecc_vpc_vpc.vepfs.vpc_id
  zone_id            = local.zone_id

  attach_file_systems = [
    {
      file_system_id = volcenginecc_vepfs_instance.first.file_system_id
      customer_path  = "/tf-vepfs-read-reorder-a"
    },
    {
      file_system_id = volcenginecc_vepfs_instance.second.file_system_id
      customer_path  = "/tf-vepfs-read-reorder-b"
    }
  ]
}

output "mount_service_id" {
  value = volcenginecc_vepfs_mount_service.read_reorder.mount_service_id
}
