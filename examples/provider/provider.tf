provider "godaddy-dns" {
  api_key    = "better set it in GODADDY_API_KEY"
  api_secret = "better set it in GODADDY_API_SECRET"
}

# create "alias.test.com" as CNAME for "other.com"
resource "godaddy-dns_record" "my-cname" {
  domain = "test.com"
  type   = "CNAME"
  name   = "alias"
  data   = "other.com"
}
