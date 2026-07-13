resource "volcenginecc_veenedge_veen" "Example" {
  cloud_server_identity   = "cloudserver-plxxxxxxxx4nr"
  create_instance_timeout = 120
  instance_area_nums = [{
    area_name             = ""
    cluster_name          = "bdcdn-gxxxxt06"
    isp                   = ""
    default_isp           = ""
    external_network_mode = ""
    vpc_identity          = "vpc-dvxxxxx659"
    subnet_identity       = "subnet-vbgxxxxxxfvq5t"
    num                   = 1
    host_name_list        = []
    single_interface_name_config = {
      external_interface_name = ""
      internal_interface_name = ""
    }
    multi_interface_name_config = {
      ctcc_external_interface_name = ""
      internal_interface_name      = ""
      cucc_external_interface_name = ""
      cmcc_external_interface_name = ""
    }
  }]
  tags = [{
    value = "test"
    key   = "env"
  }]
  instance_name = "test-instance"
  custom_data = {
    is_base_64 = false
    data       = "test data"
  }
  advanced_configuration = {
    delete_protection = true
  }
}