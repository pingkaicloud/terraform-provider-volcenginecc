resource "volcenginecc_ecs_scheduled_instance" "example" {
  image_id                = "image-xxxxxxxxxxxxxxxxxxxx"
  instance_name           = "ecs-example"
  instance_type_id        = "ecs.c3a.large"
  scheduled_instance_name = "scheduled-example"
  zone_id                 = "cn-beijing-a"

  network_interfaces = [
    {
      security_group_ids   = ["sg-xxxxxxxxxxxxxxxxxxxx"]
      subnet_id            = "subnet-xxxxxxxxxxxxxxxxxxxx"
      vpc_id               = "vpc-xxxxxxxxxxxxxxxxxxxx"
      primary_ip_address   = ""
      private_ip_addresses = []
    }
  ]

  volumes = [
    {
      size                            = 20
      volume_type                     = "ESSD_PL0"
      delete_with_instance            = false
      extra_performance_iops          = 0
      extra_performance_throughput_mb = 0
      extra_performance_type_id       = ""
      snapshot_id                     = ""
    }
  ]

  instance_count                  = 1
  min_count                       = 1
  elastic_scheduled_instance_type = "Esi"
  start_delivery_at               = "2026-12-31T02:00:00+08:00"
  end_delivery_at                 = "2026-12-31T02:10:00+08:00"
  host_name                       = "cc-test"
  unique_suffix                   = false
  suffix_index                    = 1
  project_name                    = "default"
  description                     = "instance-desc"
  scheduled_instance_description  = "desc"
  key_pair_name                   = "example-keypair"
  keep_image_credential           = false
  install_run_command_agent       = false
  deletion_protection             = false
  security_enhancement_strategy   = "Active"
  http_tokens                     = "optional"
  user_data                       = ""

  eip_address = {
    bandwidth_mbps                  = 1
    bandwidth_package_id            = ""
    charge_type                     = "PayByTraffic"
    isp                             = "BGP"
    release_with_instance           = false
    security_protection_instance_id = 0
    security_protection_types       = []
  }

  tags = [
    {
      key   = "key1"
      value = "1"
    },
    {
      key   = "key2"
      value = "2"
    },
  ]
}
