package provider

// run one test:
// TF_ACC=1 go test -count=1 -run='TestAccCnameResource' -v

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const ACC_TEST_DOM = "veksh.in"

func TestAccCnameResource(t *testing.T) {
	// alt: UnitTest (also run w/o TF_ACC=1, sets IsUnitTest = true)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// create + back
			{
				// alt: ConfigFile or ConfigDirectory
				Config: testAccExampleCnameResourceConfig("testing.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						"godaddy-dns_record.test-cname",
						"type",
						"CNAME"),
					resource.TestCheckResourceAttr(
						"godaddy-dns_record.test-cname",
						"name",
						"_test-cn._testacc"),
					resource.TestCheckResourceAttr(
						"godaddy-dns_record.test-cname",
						"data",
						"testing.com"),
				),
			},
			// import state
			{
				ResourceName:      "godaddy-dns_record.test-cname",
				ImportState:       true,
				ImportStateVerify: true,
				// ImportStateId: "veksh.in:CNAME:_test-cn._testacc:test.com",
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					attrs := s.Modules[0].Resources["godaddy-dns_record.test-cname"].Primary.Attributes
					return fmt.Sprintf("%s:%s:%s:%s",
						attrs["domain"],
						attrs["type"],
						attrs["name"],
						attrs["data"]), nil
				},
				ImportStateVerifyIdentifierAttribute: "name",
				// ImportStateVerifyIgnore: []string{"configurable_attribute", "defaulted"},
			},
			// update + read back
			{
				Config: testAccExampleCnameResourceConfig("test.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						"godaddy-dns_record.test-cname",
						"data",
						"test.com"),
				),
			},
		},
	})
}

func testAccExampleCnameResourceConfig(target string) string {
	return fmt.Sprintf(`
	provider "godaddy-dns" {}
	locals {testdomain = "%s"}
	resource "godaddy-dns_record" "test-cname" {
	  domain = "${local.testdomain}"
	  type   = "CNAME"
	  name   = "_test-cn._testacc"
	  data   = "%s"
	}`, ACC_TEST_DOM, target)
}
