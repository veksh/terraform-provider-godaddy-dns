package provider

// run one test: like
// TF_LOG=debug TF_ACC=1 go test -count=1 -run='TestAccCnameResource' -v ./internal/provider/

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/mock"
	"github.com/veksh/terraform-provider-godaddy-dns/internal/model"
)

const ACC_TEST_DOM = "veksh.in"

// go test -count=1 -run='TestUnitCnameResource' -v ./internal/provider/
func TestUnitCnameResource(t *testing.T) {
	mockClient := model.NewMockDNSApiClient(t)
	// parent.EXPECT().
	// 	StoreNotification(
	// 		mock.AnythingOfType("*context.emptyCtx"),
	// 		note20DaysAgoReq).
	// 	Return(int64(1), nil).
	// 	Once()
	rec := model.DNSRecord{
		Name: "_test-cn._testacc",
		Type: "CNAME",
		Data: "testing.com",
		TTL:  3600,
	}
	mockClient.EXPECT().AddRecords(
		mock.Anything,
		// mock.AnythingOfType("*context.emptyCtx"),
		model.DNSDomain("veksh.in"),
		[]model.DNSRecord{rec},
	).Return(nil).Once()
	mockClient.EXPECT().GetRecords(
		// mock.AnythingOfType("*context.emptyCtx"),
		mock.Anything,
		model.DNSDomain("veksh.in"),
		model.DNSRecordType("CNAME"),
		model.DNSRecordName("_test-cn._testacc"),
	).Return([]model.DNSRecord{rec}, nil)
	mockClient.EXPECT().DelRecords(
		mock.Anything,
		model.DNSDomain("veksh.in"),
		model.DNSRecordType("CNAME"),
		model.DNSRecordName("_test-cn._testacc"),
	).Return(nil).Once()
	// then: update + read back, then: delete
	testProviderFactory := map[string]func() (tfprotov6.ProviderServer, error){
		// pass test to the constructor
		"godaddy-dns": providerserver.NewProtocol6WithError(New(
			"test",
			func(apiURL, apiKey, apiSecret string) (model.DNSApiClient, error) {
				return model.DNSApiClient(mockClient), nil
			})()),
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactory,
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
		},
	})
}

// TF_LOG=debug TF_ACC=1 go test -count=1 -run='TestAccCnameResource' -v ./internal/provider/
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
