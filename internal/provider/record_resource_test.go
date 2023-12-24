package provider

// run one test:
// TF_ACC=1 go test -count=1 -run='TestAccCnameResource' -v

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const ACC_TEST_DOM = "veksh.in"

func TestAccCnameResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// create + back
			{
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
