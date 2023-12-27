package provider

// run one test: like
// TF_LOG=debug TF_ACC=1 go test -timeout 10s -run='TestAccCnameResource' -v ./internal/provider/
// go test -timeout 5s -run='TestUnitCnameResource' -v ./internal/provider/

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/veksh/terraform-provider-godaddy-dns/internal/client"
	"github.com/veksh/terraform-provider-godaddy-dns/internal/model"
)

const TEST_DOMAIN = "veksh.in"

// go test -timeout=5s -run='TestUnitCnameResource' -v ./internal/provider/
// sadly, terraform framework hangs when mock calls t.FailNow(), so short timeout is essential :)
func TestUnitCnameResource(t *testing.T) {
	resourceName := "godaddy-dns_record.test-cname"
	mockCtx := mock.AnythingOfType("*context.valueCtx")
	mockDom := model.DNSDomain(TEST_DOMAIN)
	mockRType := model.DNSRecordType("CNAME")
	mockRName := model.DNSRecordName("_test-cn._testacc")

	// add record, read it back
	// also: calls DelRecord if step fails, mb add it as optional
	mockClientAdd := model.NewMockDNSApiClient(t)
	recAdd := []model.DNSRecord{{
		Name: "_test-cn._testacc",
		Type: "CNAME",
		Data: "testing.com",
		TTL:  3600,
	}}
	mockClientAdd.EXPECT().AddRecords(mockCtx, mockDom, recAdd).Return(nil).Once()
	mockClientAdd.EXPECT().GetRecords(mockCtx, mockDom, mockRType, mockRName).Return(recAdd, nil)

	// read state
	mockClientImp := model.NewMockDNSApiClient(t)
	recImp := []model.DNSRecord{{
		Name: "_test-cn._testacc",
		Type: "CNAME",
		Data: "testing.com",
		TTL:  3600,
	}}
	mockClientImp.EXPECT().GetRecords(mockCtx, mockDom, mockRType, mockRName).Return(recImp, nil)

	// read recod, expect mismatch with saved state
	mockClientRef := model.NewMockDNSApiClient(t)
	recRef := []model.DNSRecord{{
		Name: "_test-cn._testacc",
		Type: "CNAME",
		Data: "test-upd.com",
		TTL:  3600,
	}}
	mockClientRef.EXPECT().GetRecords(mockCtx, mockDom, mockRType, mockRName).Return(recRef, nil)

	// read, update, clean up
	// also: must skip update if already ok
	mockClientUpd := model.NewMockDNSApiClient(t)
	recOrig := []model.DNSRecord{{
		Name: "_test-cn._testacc",
		Type: "CNAME",
		Data: "testing.com",
		TTL:  3600,
	}}
	rec2set := []model.DNSRecord{{
		Data: "test.com",
		TTL:  3600,
	}}
	recUpdated := []model.DNSRecord{{
		Name: "_test-cn._testacc",
		Type: "CNAME",
		Data: "test.com",
		TTL:  3600,
	}}
	// if using same args + "Once": results could vary on 1st and 2nd call
	// if more than 1 required: .Times(4)
	mockClientUpd.EXPECT().GetRecords(mockCtx, mockDom, mockRType, mockRName).Return(recOrig, nil).Once()
	mockClientUpd.EXPECT().SetRecords(mockCtx, mockDom, mockRType, mockRName, rec2set).Return(nil).Once()
	mockClientUpd.EXPECT().GetRecords(mockCtx, mockDom, mockRType, mockRName).Return(recUpdated, nil).Once()
	mockClientUpd.EXPECT().DelRecords(mockCtx, mockDom, mockRType, mockRName).Return(nil).Once()

	resource.UnitTest(t, resource.TestCase{
		// ProtoV6ProviderFactories: testProviderFactory,
		Steps: []resource.TestStep{
			// create, read back
			{
				// alt: ConfigFile or ConfigDirectory
				ProtoV6ProviderFactories: mockClientProviderFactory(mockClientAdd),
				Config:                   testCnameResourceConfig("testing.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						resourceName,
						"type",
						"CNAME"),
					resource.TestCheckResourceAttr(
						resourceName,
						"name",
						"_test-cn._testacc"),
					resource.TestCheckResourceAttr(
						resourceName,
						"data",
						"testing.com"),
				),
			},
			// read, compare with saved, should produce no plan
			{
				ProtoV6ProviderFactories: mockClientProviderFactory(mockClientImp),
				ResourceName:             resourceName,
				ImportState:              true,
				ImportStateVerify:        true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					attrs := s.Modules[0].Resources[resourceName].Primary.Attributes
					return fmt.Sprintf("%s:%s:%s:%s",
						attrs["domain"], attrs["type"], attrs["name"], attrs["data"]), nil
				},
				ImportStateVerifyIdentifierAttribute: "name",
			},
			// read, compare with saved, should produce update plan
			{
				ProtoV6ProviderFactories: mockClientProviderFactory(mockClientRef),
				ResourceName:             resourceName,
				RefreshState:             true,
				ExpectNonEmptyPlan:       true,
			},
			// update, read back
			{
				ProtoV6ProviderFactories: mockClientProviderFactory(mockClientUpd),
				Config:                   testCnameResourceConfig("test.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						resourceName,
						"data",
						"test.com"),
				),
			},
		},
	})
}

// TF_LOG=debug TF_ACC=1 go test -count=1 -run='TestAccCnameResource' -v ./internal/provider/
func TestAccCnameResource(t *testing.T) {

	apiClient, err := client.NewClient(
		GODADDY_API_URL,
		os.Getenv("GODADDY_API_KEY"),
		os.Getenv("GODADDY_API_SECRET"))
	assert.Nil(t, err, "cannot create client")

	resourceName := "godaddy-dns_record.test-cname"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		// CheckDestroy:
		Steps: []resource.TestStep{
			// create + back
			{
				// alt: ConfigFile or ConfigDirectory
				Config: testCnameResourceConfig("testing.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						resourceName,
						"type",
						"CNAME"),
					resource.TestCheckResourceAttr(
						resourceName,
						"name",
						"_test-cn._testacc"),
					resource.TestCheckResourceAttr(
						resourceName,
						"data",
						"testing.com"),
					func(s *terraform.State) error {
						attrs := s.Modules[0].Resources[resourceName].Primary.Attributes

						apiRecs, err := apiClient.GetRecords(
							context.Background(),
							model.DNSDomain(attrs["domain"]),
							model.DNSRecordType(attrs["type"]),
							model.DNSRecordName(attrs["name"]))
						if err != nil {
							t.Error("cannot get record back: client error", err)
							return err
						}
						if len(apiRecs) != 1 {
							t.Error("cannot get record back: wrong number of results", err)
							return fmt.Errorf("api check: wrong number of results")
						}
						if string(apiRecs[0].Data) != attrs["data"] {
							t.Error("wrong data attr on record", err)
							return fmt.Errorf("wrong record found")
						}
						return nil
					},
				),
			},
			// import state
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				// ImportStateId: "veksh.in:CNAME:_test-cn._testacc:test.com",
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					attrs := s.Modules[0].Resources[resourceName].Primary.Attributes
					return fmt.Sprintf("%s:%s:%s:%s",
						attrs["domain"], attrs["type"], attrs["name"], attrs["data"]), nil
				},
				ImportStateVerifyIdentifierAttribute: "name",
				// ImportStateVerifyIgnore: []string{"configurable_attribute", "defaulted"},
			},
			// update + read back
			{
				Config: testCnameResourceConfig("test.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						resourceName,
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
	}`, TEST_DOMAIN, target)
}

func mockClientProviderFactory(c *model.MockDNSApiClient) map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"godaddy-dns": providerserver.NewProtocol6WithError(New(
			"test",
			func(apiURL, apiKey, apiSecret string) (model.DNSApiClient, error) {
				return model.DNSApiClient(c), nil
			})()),
	}
}
