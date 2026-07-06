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

resource "volcenginecc_rdspostgresql_instance" "read_reorder" {
  instance_name     = "tf-pg-read-reorder"
  db_engine_version = "PostgreSQL_14"
  node_info = [
    {
      zone_id   = "cn-beijing-a"
      node_spec = "rds.postgres.1c2g"
      node_type = "Primary"
    },
    {
      zone_id   = "cn-beijing-a"
      node_spec = "rds.postgres.1c2g"
      node_type = "Secondary"
    }
  ]
  storage_type  = "LocalSSD"
  storage_space = 20
  vpc_id        = "vpc-rrco37ovjq4gv0x58zft8ul"
  subnet_id     = "subnet-rrwqhg3qzxfkv0x57g3edcq"
  charge_detail = {
    charge_type = "PostPaid"
  }
  project_name = "default"
}

output "instance_id" {
  value = volcenginecc_rdspostgresql_instance.read_reorder.instance_id
}
