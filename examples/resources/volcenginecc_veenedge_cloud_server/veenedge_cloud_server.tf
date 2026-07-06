resource "volcenginecc_veenedge_cloud_server" "Example" {
  cloud_server_name = "测试边缘服务"
  cloud_server_desc = "包年包月计费及全配置测试"
  image_id          = "imagexxxxxxiqm"
  spec_name         = "veEN.I1.2xlarge"
  project           = "default"
  disable_vga       = false
  advanced_configuration = {
    instance_host_name = "host-name"
    instance_name      = "ccapi-test-1"
    delete_protection  = true
    instance_desc      = "测试实例描述"
  }
  billing_config = {
    bandwidth_billing_method = "MonthlyP95"
    computing_billing_method = "MonthlyPeak"
  }
  custom_data = {
    data = "asfdsadfasdf"
  }
  network_config = {
    bandwidth_peak                 = "5"
    bound_eip_share_bandwidth_peak = "5"
    custom_external_interface_name = "eth1"
    custom_internal_interface_name = "eth0"
    dns_list                       = ["114.114.114.114", "180.184.1.1", "223.6.6.7"]
    dns_type                       = "custom"
    enable_ipv_6                   = true
    limit_mode                     = "shared"
    secondary_internal_ip_num      = 1
    security_group_id_list         = ["veew-sg-113xxxxxxxx195"]
    tcp_timeout                    = 900
    udp_timeout                    = 60
  }
  schedule_strategy = {
    price_strategy    = "low_priority"
    schedule_strategy = "dispersion"
  }
  secret_config = {
    secret_type = 3
    secret_data = "sshkey-dzxxxxxxx78xx"
  }
  storage_config = {
    system_disk = {
      capacity     = "40"
      storage_type = "CloudBlockSSD"
    }
    data_local_disks = [{
      num = 1
      disk_spec = {
        capacity     = "950"
        storage_type = "LocalSSD"
      }
    }]
  }
  tags = [{
    value = "env"
    key   = "test"
  }]
}