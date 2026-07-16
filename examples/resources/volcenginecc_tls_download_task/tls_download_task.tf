resource "volcenginecc_tls_download_task" "Example" {
  topic_id         = "22fca26e-xxxxx-a3ca-a9bb6d3fb9bd"
  task_name        = "task-saie"
  query            = ""
  start_time       = 1767196800000
  end_time         = 1783585260783
  compression      = "gzip"
  data_format      = "json"
  limit            = 2000
  sort             = "desc"
  task_type        = 0
  allow_incomplete = false
  log_context_infos = {
    source         = ""
    context_flow   = ""
    package_offset = 0
  }
  must_complete = true

}