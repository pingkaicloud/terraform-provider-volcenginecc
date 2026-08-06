resource "volcenginecc_certificateservice_child_cert_instance" "primary_certificateservice_childcertinstance_case_3" {
  tag                     = "dx-cert-2"
  project_name            = "default"
  repeatable              = true
  no_verify_and_fix_chain = false
  import_certificate_info = {
    certificate_chain = "-----BEGIN CERTIFICATE-----\nMIID0EXAMPLECERTIFICATECONTENT\n-----END CERTIFICATE-----"
    private_key       = "-----BEGIN RSA PRIVATE KEY-----\nMIIEoEXAMPLEPRIVATEKEYCONTENT\n-----END RSA PRIVATE KEY-----"
  }
  tags = [{
    key   = "aaa"
    value = "111"
  }]
}
