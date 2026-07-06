resource "volcenginecc_id_permission_namespace" "Example" {
  namespace_name = "test-namespace-full"
  description    = "test description"
  project_name   = "default"
  tags = [{
    key   = "env"
    value = "test"
  }]
}