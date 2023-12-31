# GoDaddy DNS Terraform Provider

This module allows to manage individual DNS resource records for domains
hosted on GoDaddy DNS servers, using [management API](https://developer.godaddy.com/).

It manages only DNS resources (no e.g. domain management) and aim to manage
individual DNS resource records (not the whole domain), while preserving existant
records and tolerating external modifications (as far as possible).

Example usage is pretty straightforward

``` HCL
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

It currently supports `A`, `AAAA`, `CNAME`, `MX`, `NS` and `TXT` records, the only
omission being `SRV` (if anyone hosting AD on GoDaddy or uses them for VOIP or
something like that, please let me know by creating an issue).

Differences vs n3integration provider and its forks are
- granularity: top-level configuration object is record, not domain
- modifications do not result in scary plans to destroy the whole domain
- `destroy` is fully supported
