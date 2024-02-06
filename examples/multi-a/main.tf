terraform {
  required_providers {
    godaddy-dns = {
      source = "registry.terraform.io/veksh/godaddy-dns"
    }
  }
}

provider "godaddy-dns" {}

locals {
  domain     = "veksh.in"
  subDomain  = "man-test"
  recName    = "multi-a"
  dataValues = ["3.3.3.3", "4.4.4.4"]
}

# terraform import 'godaddy-dns_record.as[0]' veksh.in:A:multi-a.man-test:1.1.1.1
resource "godaddy-dns_record" "as" {
  domain = local.domain
  type   = "A"
  name   = "${local.recName}.${local.subDomain}"

  # add is always ok, mod/del could be problematic
  # conservative: terraform plan -destroy -parallelism=1
  count = length(local.dataValues)
  data  = local.dataValues[count.index]
  ttl   = 600
}
