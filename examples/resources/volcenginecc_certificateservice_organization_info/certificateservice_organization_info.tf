resource "volcenginecc_certificateservice_organization_info" "example" {
  type = "Individual"
  tag  = "tf-org-individual-tags"

  contact = {
    first_name = "Example"
    last_name  = "User"
    email      = "user@example.com"
    phone      = "00000000000"
    id_card_no = "ID_CARD_NUMBER"
  }

  project_name = "default"

  tags = [
    {
      key   = "case"
      value = "individual-tags"
    },
    {
      key   = "env"
      value = "test"
    }
  ]
}
