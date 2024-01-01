# GoDaddy DNS provider for Terraform

This plug-in enables the managment of individual DNS resource records for domains hosted on GoDaddy DNS servers, using [the management API](https://developer.godaddy.com/).

It only manages DNS resources (no e.g. domain management) and aims to manage individual DNS resource records (not the whole domain), while preserving existant records and tolerating external modifications.

## Usage

Example usage (set credentials in `GODADDY_API_KEY` and `GODADDY_API_SECRET` env vars):

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

## Supported RR types, mode of operation

It currently supports `A`, `AAAA`, `CNAME`, `MX`, `NS` and `TXT` records. `SRV` are not supported; if anyone hosting AD on GoDaddy or uses them for VOIP or something like that, please let me know by creating an issue.

GoDaddy API does not have stable identities for DNS records, and in case of external modifications (e.g. via web console) behavior is slightly different for "single-valued" vs "mult-valued" records
- for "single-valued" record types (`A` and `CNAME`) there could be only 1 record of this type with a given name, so these are just replaced by update
- for "multi-valued" record types (`MX`, `NS`, `TXT`) there could be several records with a given name (e.g. multiple MXes with different priorities and targets), so matching is done on value; if record's value is modified outside of Terraform, it is treated as a completely new record and is preserved (and original record is considered gone), so new record is created on update.

## Differences vs alternative providers

Differences vs n3integration provider and its forks:
- granularity: top-level configuration object is record, not domain
- updates do not result in scary plans threatening to destroy all the records in whole domain
- external modifications are mostly ok: update will not complain if record is already set to the desired state
- `destroy` is fully supported
