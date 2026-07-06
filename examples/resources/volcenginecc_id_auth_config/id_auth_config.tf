resource "volcenginecc_id_auth_config" "Example" {
  config_name = "测试JWT认证配置"
  description = "用于测试的JWT入站认证配置"
  instance_id = "example"
  jwt_auth_config = {
    discovery_url     = "https://example1.com/.well-known/openid-configuration"
    allowed_audiences = ["api.example.com", "mobile.example.com"]
    allowed_clients   = ["web-client", "app-client"]
  }
  auth_type = "Jwt"
}