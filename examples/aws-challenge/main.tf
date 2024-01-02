locals {
  website_name   = "mysite"
  website_domain = "mydomain.com"
}

# cloudfront requires (custom) cert to be in "us-east-1" region
provider "aws" {
  alias  = "us-east-1"
  region = "us-east-1"
}

# expects GODADDY_API_KEY and GODADDY_API_SECRET in environment
provider "godaddy-dns" {}

resource "aws_acm_certificate" "nondefault_cert" {
  provider          = aws.us-east-1
  domain_name       = local.website_name
  validation_method = "DNS"
  lifecycle {
    create_before_destroy = true
  }
}

resource "godaddy-dns_record" "cert_challenge" {
  for_each = {
    for dvo in aws_acm_certificate.nondefault_cert.domain_validation_options :
    dvo.domain_name => {
      name = trimsuffix(dvo.resource_record_name, join("", [".", local.website_domain, "."]))
      data = trimsuffix(dvo.resource_record_value, ".")
    }
  }

  domain = local.website_domain
  type   = "CNAME"
  name   = each.value.name
  data   = each.value.data
}

resource "aws_acm_certificate_validation" "nondefault_cert_valid" {
  provider        = aws.us-east-1
  certificate_arn = aws_acm_certificate.nondefault_cert.arn
  validation_record_fqdns = [
    for dvo in aws_acm_certificate.nondefault_cert.domain_validation_options :
  dvo.resource_record_name]
}

resource "aws_cloudfront_distribution" "s3_distribution" {
  aliases = [local.website_name]
  viewer_certificate {
    acm_certificate_arn = aws_acm_certificate_validation.nondefault_cert_valid.certificate_arn
    ssl_support_method  = "sni-only"
  }

  # rest of config is skipped, these are only for lint to stop complaining
  enabled = true
  restrictions {
    geo_restriction {
      restriction_type = ""
    }
  }
  origin {
    origin_id   = 1
    domain_name = ""
  }
  default_cache_behavior {
    allowed_methods        = [""]
    cached_methods         = [""]
    target_origin_id       = ""
    viewer_protocol_policy = ""
  }
}
