terraform {
  required_providers {
    godaddy-dns = {
      source = "registry.terraform.io/veksh/godaddy-dns"
    }
  }
}

# keys from env
provider "godaddy-dns" {}

# struct for several records
locals {
  records = {
    "mx" = {
      type = "MX",
      name = "_test-cli",
      data = "mx1.pseudo.com",
      prio = 10,
    },
    "txt" = {
      type = "TXT",
      name = "_test-cli",
      data = "also, txt",
    },
  }
}

# existing: import like
# terraform import godaddy-dns_record.new-cname domain:CNAME:_test-cn:testing.com
resource "godaddy-dns_record" "array-of-records" {
  for_each = local.records
  domain   = "veksh.in"
  type     = each.value.type
  name     = each.value.name
  data     = each.value.data
  priority = lookup(each.value, "prio", null)
}
