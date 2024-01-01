# GoDaddy DNS provider for Terraform

This plug-in enables the managment of individual DNS resource records for domains hosted on GoDaddy DNS servers, using [the management API](https://developer.godaddy.com/).

It only manages DNS resources (no e.g. domain management) and aims to manage individual DNS resource records (not the whole domain), while preserving existant records and tolerating external modifications.

Example usage (assuming environment variables `GODADDY_API_KEY` and `GODADDY_API_SECRET` are set up with GoDaddy API credentials):

``` terraform
terraform {
  required_providers {
    godaddy-dns = {
      source = "registry.terraform.io/veksh/godaddy-dns"
    }
  }
}

# keys usually set with env GODADDY_API_KEY and GODADDY_API_SECRET
provider "godaddy-dns" {}

# to import existing records:
# terraform import godaddy-dns_record.new-cname domain.com:CNAME:alias:testing.com
resource "godaddy-dns_record" "new-cname" {
  domain = "domain.com"
  type   = "CNAME"
  name   = "alias"
  data   = "test.com"
}
```

It currently supports `A`, `AAAA`, `CNAME`, `MX`, `NS` and `TXT` records (no `SRV` yet -- if anyone hosting AD on GoDaddy or uses them for VOIP or something like that, please let me know by creating an issue).

Differences vs n3integration provider and its forks:
- granularity: top-level configuration object is record, not domain
- modifications do not result in scary plans to destroy the whole domain
- `destroy` is fully supported
