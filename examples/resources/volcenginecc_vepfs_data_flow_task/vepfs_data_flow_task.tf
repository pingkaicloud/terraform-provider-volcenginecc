resource "volcenginecc_vepfs_data_flow_task" "example" {
  file_system_id        = "vepfs-xxxxxx"
  task_action           = "Import"
  data_type             = "MetaAndData"
  data_storage          = "example-bucket"
  sub_path              = "/"
  same_name_file_policy = "KeepLatest"

  entry_list_file_info = {
    file_name   = "data.csv"
    file_bucket = "example-bucket"
    file_key    = "data.csv"
  }
}
