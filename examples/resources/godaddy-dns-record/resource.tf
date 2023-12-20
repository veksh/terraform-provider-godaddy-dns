resource "godaddy_dns_record" "cname" {
  domain = "mydomain.com"

  type = "CNAME"
  name = "redirect"
  data = "target.otherdomain.com"
}
