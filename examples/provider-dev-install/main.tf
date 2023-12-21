terraform {
  required_providers {
    godaddy-dns = {
      source = "registry.terraform.io/veksh/godaddy-dns"
    }
  }
}

provider "godaddy-dns" {
  api_key    = "dummy"
  api_secret = "pseudo"
}

data "godaddy-dns_empty" "test" {}
