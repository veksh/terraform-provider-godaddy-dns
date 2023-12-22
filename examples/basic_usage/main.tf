terraform {
  required_providers {
    godaddy-dns = {
      source = "registry.terraform.io/veksh/godaddy-dns"
    }
  }
}

# keys from env
provider "godaddy-dns" {}

# existing: import like
# terraform import godaddy-dns_record.new-cname domain:CNAME:_test-cn:testing.com
resource "godaddy-dns_record" "new-cname" {
  domain = "veksh.in"
  type   = "CNAME"
  name   = "_test-cn"
  data   = "testing.com"
}
