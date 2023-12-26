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
	// add record, read it back
	// also: calls DelRecord if step fails, mb add it + make optional
	mockClientAdd := model.NewMockDNSApiClient(t)
	recAdd := model.DNSRecord{
		Name: "_test-cn._testacc",
		Type: "CNAME",
		Data: "testing.com",
		TTL:  3600,
	}
	mockClientAdd.EXPECT().AddRecords(
		mock.AnythingOfType("*context.valueCtx"),
		model.DNSDomain("veksh.in"),
		[]model.DNSRecord{recAdd},
	).Return(nil).Once()
	mockClientAdd.EXPECT().GetRecords(
		mock.AnythingOfType("*context.valueCtx"),
		model.DNSDomain("veksh.in"),
		model.DNSRecordType("CNAME"),
		model.DNSRecordName("_test-cn._testacc"),
	).Return([]model.DNSRecord{recAdd}, nil)
	testProviderFactoryAdd := map[string]func() (tfprotov6.ProviderServer, error){
		// pass test to the constructor
		"godaddy-dns": providerserver.NewProtocol6WithError(New(
			"test",
			func(apiURL, apiKey, apiSecret string) (model.DNSApiClient, error) {
				return model.DNSApiClient(mockClientAdd), nil
			})()),
	}

	// read state
	mockClientImp := model.NewMockDNSApiClient(t)
	recImp := model.DNSRecord{
		Name: "_test-cn._testacc",
		Type: "CNAME",
		Data: "testing.com",
		TTL:  3600,
	}
	mockClientImp.EXPECT().GetRecords(
		mock.AnythingOfType("*context.valueCtx"),
		model.DNSDomain("veksh.in"),
		model.DNSRecordType("CNAME"),
		model.DNSRecordName("_test-cn._testacc"),
	).Return([]model.DNSRecord{recImp}, nil)
	testProviderFactoryImp := map[string]func() (tfprotov6.ProviderServer, error){
		// pass test to the constructor
		"godaddy-dns": providerserver.NewProtocol6WithError(New(
			"test",
			func(apiURL, apiKey, apiSecret string) (model.DNSApiClient, error) {
				return model.DNSApiClient(mockClientImp), nil
			})()),
	}

	// read recod, expect mismatch with saved state
	mockClientRef := model.NewMockDNSApiClient(t)
	recRef := model.DNSRecord{
		Name: "_test-cn._testacc",
		Type: "CNAME",
		Data: "test-upd.com",
		TTL:  3600,
	}
	mockClientRef.EXPECT().GetRecords(
		mock.AnythingOfType("*context.valueCtx"),
		model.DNSDomain("veksh.in"),
		model.DNSRecordType("CNAME"),
		model.DNSRecordName("_test-cn._testacc"),
	).Return([]model.DNSRecord{recRef}, nil)
	testProviderFactoryRef := map[string]func() (tfprotov6.ProviderServer, error){
		// pass test to the constructor
		"godaddy-dns": providerserver.NewProtocol6WithError(New(
			"test",
			func(apiURL, apiKey, apiSecret string) (model.DNSApiClient, error) {
				return model.DNSApiClient(mockClientRef), nil
			})()),
	}

	// read, update, clean up
	// also: must skip update if already ok
	mockClientUpd := model.NewMockDNSApiClient(t)
	recOrig := model.DNSRecord{
		Name: "_test-cn._testacc",
		Type: "CNAME",
		Data: "testing.com",
		TTL:  3600,
	}
	rec2set := model.DNSRecord{
		Data: "test.com",
		TTL:  3600,
	}
	recUpdated := model.DNSRecord{
		Name: "_test-cn._testacc",
		Type: "CNAME",
		Data: "test.com",
		TTL:  3600,
	}
	// if using same args + "Once": results could vary on 1st and 2nd call
	mockClientUpd.EXPECT().GetRecords(
		mock.AnythingOfType("*context.valueCtx"),
		model.DNSDomain("veksh.in"),
		model.DNSRecordType("CNAME"),
		model.DNSRecordName("_test-cn._testacc"),
	).Return([]model.DNSRecord{recOrig}, nil).Once()
	mockClientUpd.EXPECT().SetRecords(
		mock.AnythingOfType("*context.valueCtx"),
		model.DNSDomain("veksh.in"),
		model.DNSRecordType("CNAME"),
		model.DNSRecordName("_test-cn._testacc"),
		[]model.DNSRecord{rec2set},
	).Return(nil).Once()
	mockClientUpd.EXPECT().GetRecords(
		mock.AnythingOfType("*context.valueCtx"),
		model.DNSDomain("veksh.in"),
		model.DNSRecordType("CNAME"),
		model.DNSRecordName("_test-cn._testacc"),
	).Return([]model.DNSRecord{recUpdated}, nil).Once()
	mockClientUpd.EXPECT().DelRecords(
		mock.AnythingOfType("*context.valueCtx"),
		model.DNSDomain("veksh.in"),
		model.DNSRecordType("CNAME"),
		model.DNSRecordName("_test-cn._testacc"),
	).Return(nil).Once()
	testProviderFactoryUpd := map[string]func() (tfprotov6.ProviderServer, error){
		// pass test to the constructor
		"godaddy-dns": providerserver.NewProtocol6WithError(New(
			"test",
			func(apiURL, apiKey, apiSecret string) (model.DNSApiClient, error) {
				return model.DNSApiClient(mockClientUpd), nil
			})()),
	}

	resource.UnitTest(t, resource.TestCase{
		// ProtoV6ProviderFactories: testProviderFactory,
		Steps: []resource.TestStep{
			// create, read back
			{
				// alt: ConfigFile or ConfigDirectory
				ProtoV6ProviderFactories: testProviderFactoryAdd,
				Config:                   testCnameResourceConfig("testing.com"),
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
			// read, compare with saved, should produce no plan
			{
				ProtoV6ProviderFactories: testProviderFactoryImp,
				ResourceName:             "godaddy-dns_record.test-cname",
				ImportState:              true,
				ImportStateVerify:        true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					attrs := s.Modules[0].Resources["godaddy-dns_record.test-cname"].Primary.Attributes
					return fmt.Sprintf("%s:%s:%s:%s",
						attrs["domain"],
						attrs["type"],
						attrs["name"],
						attrs["data"]), nil
				},
				ImportStateVerifyIdentifierAttribute: "name",
			},
			// read, compare with saved, should produce update plan
			{
				ProtoV6ProviderFactories: testProviderFactoryRef,
				ResourceName:             "godaddy-dns_record.test-cname",
				RefreshState:             true,
				ExpectNonEmptyPlan:       true,
			},
			// update, read back
			{
				ProtoV6ProviderFactories: testProviderFactoryUpd,
				Config:                   testCnameResourceConfig("test.com"),
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
				Config: testCnameResourceConfig("testing.com"),
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
				Config: testCnameResourceConfig("test.com"),
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

func testCnameResourceConfig(target string) string {
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
