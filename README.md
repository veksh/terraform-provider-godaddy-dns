# GoDaddy DNS provider for Terraform

This plug-in enables the management of individual DNS resource records for domains hosted on GoDaddy DNS servers.

It only manages DNS (no e.g. domain management) and aims to manage individual DNS resource records (not the whole domain), while preserving existent records and tolerating external modifications.

## GoDaddy API access restrictions and the end of active development for this provider

Unfortunately, at the start of May 2024 GoDaddy suddenly decided to restrict access to their DNS management API: all API calls started to fail with the cryptic error message ("Authenticated user is not allowed access"). There were no official announcements or explanation for a while, but eventually they updated the [documentation page](https://developer.godaddy.com/getstarted#apiaccess) with the new requirements: DNS API access is now available only to accounts with 10 or more registered domains, or having an active Discount Domain Club Premier Membership plan. Currently I have neither, and so I cannot use API for domain management or for integration testing. Thus, active development of this provider is stopped. It will continue to work the API itself will change in incompatible way. According to the same document, this could happen ["at any time and for any reason without any prior notice or liability to you"](https://developer.godaddy.com/getstarted#apichange), so caveat emptor.

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

GoDaddy API does not have stable identities for DNS records, and in case of external modifications (e.g. via web console) behaviour is slightly different for "single-valued" vs "multi-valued" records
- for "single-valued" record types (`CNAME`) there could be only 1 record of this type with a given name, so these are just replaced by update
- for "multi-valued" record types (`A`, `MX`, `NS`, `TXT`) there could be several records
with a given name (e.g. multiple MXes with different priorities and targets), so matching is done on value
- if record's value is modified outside of Terraform, it is treated as a completely different record and is preserved, while original record is considered gone and is re-created on `apply` (use `refresh` + `import` to re-link modified record back to original).

## Differences vs alternative providers

Differences vs n3integration provider and its forks:
- configuration object is record, not (top-level) domain no need to bring it all under terraform management
- could safely manage a subset of records (e.g. only one TXT or CNAME at domain top), keeping the rest intact
- updates do not result in scary plans threatening to destroy all the records in the whole domain
- `destroy` is fully supported and removes only previously created records
