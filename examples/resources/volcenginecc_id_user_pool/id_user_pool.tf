resource "volcenginecc_id_user_pool" "Example" {
  name        = "禁用开关的用户池"
  description = "这是一个禁用开关的用户池"
  brand = {
    logo_uri = "https://example.com/logo.png"
    name     = "测试品牌"
  }
  password_sign_in_enabled           = false
  sms_anonymous_sign_up_enabled      = false
  email_passwordless_sign_in_enabled = false
  self_sign_up_enabled               = false
  sign_in_attributes                 = ["preferred_username", "phone_number", "email"]
  required_sign_up_attributes        = ["preferred_username", "phone_number", "email"]
  project_name                       = "default"
  sign_up_auto_verification_enabled  = false
  self_account_recovery_enabled      = false
  unconfirmed_user_sign_in_enabled   = false
  sms_passwordless_sign_in_enabled   = false
  tags = [{
    key   = "env"
    value = "test"
  }]
}