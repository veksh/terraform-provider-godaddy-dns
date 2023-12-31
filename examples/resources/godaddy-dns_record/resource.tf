# one record
resource "godaddy-dns_record" "one-cname" {
  domain = "mydomain.com"
  type   = "CNAME"
  name   = "redirect"
  data   = "target.otherdomain.com"
}

locals {
  records = {
    "mx1" = {
      type = "MX",
      name = "@",
      data = "mx01.mail.icloud.com",
      prio = 10,
    },
    "mx2" = {
      type = "MX",
      name = "@",
      data = "mx02.mail.icloud.com",
      prio = 10,
    },
    "spf" = {
      type = "TXT",
      name = "@",
      data = "\"v=spf1 include:icloud.com ~all\""
    },
  }
}

# with names like `godaddy-dns_record.records["mx"]`
resource "godaddy-dns_record" "records" {
  for_each = local.records
  domain   = "mydomain.com"
  type     = each.value.type
  name     = each.value.name
  data     = each.value.data
  priority = lookup(each.value, "prio", null)
}
