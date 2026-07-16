resource "volcenginecc_vmp_integration_task" "Example" {
  name         = "tf_test_1001"
  workspace_id = "f2e31ff4-01e2-4cef-8da5-4bxxxxxx"
  type         = "ECS"
  environment  = "Managed"
  create_params = jsonencode(
    {
      "VPCId" : "vpc-25oeebv0e4g06pyvxxxxxx",
      "SubnetIds" : [
        "subnet-2ouq84mwautc06oqj0xxxxxx"
      ],
      "SecurityGroupIds" : [
        "sg-25oeevlbio746pyvxxxxxx"
      ],
      "EnableSubnetFilter" : true,
      "ScrapeConfig" : "global:\n  scrape_interval: 15s\n  scrape_timeout: 10s\nscrape_configs:\n- job_name: ecs\n  scheme: http\n  metrics_path: /metrics\n  volc_sd_configs:\n  - port: 9091"
    }
  )
  enabled = false
  tags = [
    {
      key   = "env"
      value = "test"
    }
  ]
}