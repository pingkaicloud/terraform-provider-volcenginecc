resource "volcenginecc_rdsmysql_instance_readonly_node" "Example" {
  instance_id            = "mysql-41xxxxx4db8"
  node_spec              = "rds.mysql.d1.n.1c1g"
  zone_id                = "cn-beijing-a"
  update_endpoint_ids    = ["mysql-41xxxxx4db8-cluster"]
  delay_replication_time = 300
}