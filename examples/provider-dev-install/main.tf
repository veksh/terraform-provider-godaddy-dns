terraform {
  required_providers {
    godaddy-dns = {
      source = "registry.terraform.io/veksh/godaddy-dns"
    }
  }
}

provider "godaddy-dns" {}
