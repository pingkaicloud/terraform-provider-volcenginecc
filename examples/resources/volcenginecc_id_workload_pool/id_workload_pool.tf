resource "volcenginecc_id_workload_pool" "Example" {
  description        = "用于测试的工作负载池"
  project_name       = "default"
  workload_pool_name = "demo-workload-pool-full"
  tags = [{
    key   = "env"
    value = "test"
  }]
}