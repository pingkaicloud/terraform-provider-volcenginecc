resource "volcenginecc_id_oauth2_credential_provider" "Example" {
  config = {
    client_id     = "ClientId"
    client_secret = "ClientSecret"
    custom_parameters = {
      entries = [{
        key   = "k1"
        value = "v1"
        }, {
        key   = "k2"
        value = "v2"
      }]
    }
    flow                 = "USER_FEDERATION"
    force_authentication = false
    max_expires          = 3600
    metadata             = "test for metadata"
    oauth_2_discovery = {
      authorization_server_metadata = {
        authorization_endpoint           = "http://abc.login.com"
        code_challenge_methods_supported = ["S256"]
        issuer                           = "http://abc.user.com"
        registration_endpoint            = "http://abc.com/register"
        response_types                   = ["code", "token", "id_token", "code id_token"]
        token_endpoint                   = "http://abc.token.com"
      }
    }
    redirect_url = "http://abc.callback.com"
    scopes       = ["openid"]
  }
  name         = "ccapi-dx-1"
  pool_name    = "default"
  project_name = "default"
  vendor       = 0
}
