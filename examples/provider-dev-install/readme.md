- build with makefile or `go build -o ./bin/terraform-provider-godaddy-dns`
- add to `~/.terraformrc`
``` hcl
provider_installation {
  dev_overrides {
      "registry.terraform.io/veksh/godaddy-dns" = "/Users/alex/works/terraform/terraform-provider-godaddy-dns/bin"
  }
  direct {}
}
```
- create `main.tf` somewhere 
``` hcl
terraform {
  required_providers {
    godaddy-dns = {
      source = "registry.terraform.io/veksh/godaddy-dns"
    }
  }
}

provider "godaddy-dns" {}
```
- test with `terraform plan`
