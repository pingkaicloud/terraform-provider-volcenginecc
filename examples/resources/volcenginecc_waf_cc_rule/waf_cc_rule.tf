resource "volcenginecc_waf_cc_rule" "primary_waf_ccrule_case_2" {
  field            = "HEADER:Authorization"
  cc_type          = 2
  count_time       = 300
  enable           = 1
  single_threshold = 30
  host             = "www.testwaf.com"
  path_threshold   = 300
  rule_priority    = 9
  cron_enable      = 1
  cron_confs = [{
    single_threshold = 10
    path_threshold   = 100
    crontab          = "* 18-20 * * 1,2,3,4,5"
  }]
  url  = "/admin"
  name = "test-rule-block"

}