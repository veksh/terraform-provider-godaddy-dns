package provider

// run one test: like
// TF_LOG=debug TF_ACC=1 go test -timeout 10s -run='TestAccCnameResource' -v ./internal/provider/
// go test -timeout 5s -run='TestUnitCnameResource' -v ./internal/provider/

import (
	"context"
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/veksh/terraform-provider-godaddy-dns/internal/client"
	"github.com/veksh/terraform-provider-godaddy-dns/internal/model"
)

const TEST_DOMAIN = "veksh.in"

// TF_LOG=debug go test -timeout=5s -run='TestUnitTXTResourceWithAnother' -v ./internal/provider/
func TestUnitTXTResourceWithAnother(t *testing.T) {
	// TODO
	// - update if already ok (state is different, but read same record)
	// common fixtures
	resourceName := "godaddy-dns_record.test-txt"
	mockCtx := mock.AnythingOfType("*context.valueCtx")
	mockDom := model.DNSDomain(TEST_DOMAIN)
	mockRType := model.REC_TXT
	mockRName := model.DNSRecordName("test-txt._test")
	mockRec := model.DNSRecord{
		Name: "test-txt._test",
		Type: "TXT",
		Data: "test text",
		TTL:  3600}
	mockRecAnother := model.DNSRecord{
		Name: "test-txt._test",
		Type: "TXT",
		Data: "do not modify",
		TTL:  600}
	mockRecs := []model.DNSRecord{mockRec, mockRecAnother}
	mockRecYetAnother := model.DNSRecord{
		Name: "test-txt._test",
		Type: "TXT",
		Data: "also appears",
		TTL:  7200}

	// add record, read it back
	// also: calls DelRecord if step fails, mb add it as optional
	mockClientAdd := model.NewMockDNSApiClient(t)
	mockClientAdd.EXPECT().AddRecords(mockCtx, mockDom, []model.DNSRecord{mockRec}).Return(nil).Once()
	mockClientAdd.EXPECT().GetRecords(mockCtx, mockDom, mockRType, mockRName).Return(mockRecs, nil)

	// read state
	mockClientImp := model.NewMockDNSApiClient(t)
	mockClientImp.EXPECT().GetRecords(mockCtx, mockDom, mockRType, mockRName).Return(mockRecs, nil)

	// read recod (similate changes, so rec not found), expect mismatch with saved state
	// also: tries to destroy refreshed RR if last in pipeline; mostly ok
	mockClientRef := model.NewMockDNSApiClient(t)
	mockRecsRefresh := slices.Clone(mockRecs)
	mockRecsRefresh[0].Data = "changed text"
	mockClientRef.EXPECT().GetRecords(mockCtx, mockDom, mockRType, mockRName).Return(mockRecsRefresh, nil)

	// read (simulate another record added, and ours still present), update
	// final step: clean up (not delete but set with 2 remaining records)
	mockClientUpd := model.NewMockDNSApiClient(t)
	rec2set := []model.DNSRecord{{Data: "do not modify", TTL: 600}, {Data: "updated text", TTL: 3600}}
	mockRecsUpdated := slices.Clone(mockRecs)
	mockRecsUpdated[0].Data = "updated text"
	mockRecsUpdated = append(mockRecsUpdated, mockRecYetAnother)
	recs2keep := []model.DNSRecord{{Data: "do not modify", TTL: 600}, {Data: "also appears", TTL: 7200}}
	// 2 gets: 1st for read/refresh, 2nd for uptate/find recs to keep
	mockClientUpd.EXPECT().GetRecords(mockCtx, mockDom, mockRType, mockRName).Return(mockRecs, nil).Twice()
	mockClientUpd.EXPECT().SetRecords(mockCtx, mockDom, mockRType, mockRName, rec2set).Return(nil).Once()
	// same thing with delete: refresh, enumerate recs to keep
	mockClientUpd.EXPECT().GetRecords(mockCtx, mockDom, mockRType, mockRName).Return(mockRecsUpdated, nil).Twice()
	mockClientUpd.EXPECT().SetRecords(mockCtx, mockDom, mockRType, mockRName, recs2keep).Return(nil).Once()

	resource.UnitTest(t, resource.TestCase{
		// ProtoV6ProviderFactories: testProviderFactory,
		Steps: []resource.TestStep{
			// create, read back
			{
				// alt: ConfigFile or ConfigDirectory
				ProtoV6ProviderFactories: mockClientProviderFactory(mockClientAdd),
				Config:                   testTXTResourceConfig("test text"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						resourceName,
						"type",
						"TXT"),
					resource.TestCheckResourceAttr(
						resourceName,
						"name",
						"test-txt._test"),
					resource.TestCheckResourceAttr(
						resourceName,
						"data",
						"test text"),
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
			// update, read back, clean up (keeping others)
			{
				ProtoV6ProviderFactories: mockClientProviderFactory(mockClientUpd),
				Config:                   testTXTResourceConfig("updated text"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						resourceName,
						"data",
						"updated text"),
				),
			},
		},
	})
}

// TF_LOG=debug go test -timeout=5s -run='TestUnitTXTResource' -v ./internal/provider/
func TestUnitTXTResourceAlone(t *testing.T) {
	// TODO
	// - update if already ok (state is different, but read same record)
	// common fixtures
	resourceName := "godaddy-dns_record.test-txt"
	mockCtx := mock.AnythingOfType("*context.valueCtx")
	mockDom := model.DNSDomain(TEST_DOMAIN)
	mockRType := model.REC_TXT
	mockRName := model.DNSRecordName("test-txt._test")
	mockRec := []model.DNSRecord{{
		Name: "test-txt._test",
		Type: "TXT",
		Data: "test text",
		TTL:  3600,
	}}

	// add record, read it back
	// also: calls DelRecord if step fails, mb add it as optional
	mockClientAdd := model.NewMockDNSApiClient(t)
	mockClientAdd.EXPECT().AddRecords(mockCtx, mockDom, mockRec).Return(nil).Once()
	mockClientAdd.EXPECT().GetRecords(mockCtx, mockDom, mockRType, mockRName).Return(mockRec, nil)

	// read state
	mockClientImp := model.NewMockDNSApiClient(t)
	mockClientImp.EXPECT().GetRecords(mockCtx, mockDom, mockRType, mockRName).Return(mockRec, nil)

	// read recod, expect mismatch with saved state
	// also: tries to destroy refreshed RR if last in pipeline; mostly ok
	mockClientRef := model.NewMockDNSApiClient(t)
	mockRecRefresh := slices.Clone(mockRec)
	mockRecRefresh[0].Data = "changed text"
	mockClientRef.EXPECT().GetRecords(mockCtx, mockDom, mockRType, mockRName).Return(mockRecRefresh, nil)

	// read, update, clean up
	// also: must skip update if already ok
	mockClientUpd := model.NewMockDNSApiClient(t)
	rec2set := []model.DNSRecord{{Data: "updated text", TTL: 3600}}
	mockRecUpdated := slices.Clone(mockRec)
	mockRecUpdated[0].Data = "updated text"
	// need to return it 2 times: 1st for read (refresh), 2nd for uptate (keeping recs)
	mockClientUpd.EXPECT().GetRecords(mockCtx, mockDom, mockRType, mockRName).Return(mockRec, nil).Times(2)
	mockClientUpd.EXPECT().SetRecords(mockCtx, mockDom, mockRType, mockRName, rec2set).Return(nil).Once()
	// same thing with delete
	mockClientUpd.EXPECT().GetRecords(mockCtx, mockDom, mockRType, mockRName).Return(mockRecUpdated, nil).Times(2)
	mockClientUpd.EXPECT().DelRecords(mockCtx, mockDom, mockRType, mockRName).Return(nil).Once()

	resource.UnitTest(t, resource.TestCase{
		// ProtoV6ProviderFactories: testProviderFactory,
		Steps: []resource.TestStep{
			// create, read back
			{
				// alt: ConfigFile or ConfigDirectory
				ProtoV6ProviderFactories: mockClientProviderFactory(mockClientAdd),
				Config:                   testTXTResourceConfig("test text"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						resourceName,
						"type",
						"TXT"),
					resource.TestCheckResourceAttr(
						resourceName,
						"name",
						"test-txt._test"),
					resource.TestCheckResourceAttr(
						resourceName,
						"data",
						"test text"),
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
			// update, read back, clean up
			{
				ProtoV6ProviderFactories: mockClientProviderFactory(mockClientUpd),
				Config:                   testTXTResourceConfig("updated text"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						resourceName,
						"data",
						"updated text"),
				),
			},
		},
	})
}

// go test -timeout=5s -run='TestUnitCnameResource' -v ./internal/provider/
// sadly, terraform framework hangs when mock calls t.FailNow(), so short timeout is essential :)
func TestUnitCnameResource(t *testing.T) {
	// common fixtures
	resourceName := "godaddy-dns_record.test-cname"
	mockCtx := mock.AnythingOfType("*context.valueCtx")
	mockDom := model.DNSDomain(TEST_DOMAIN)
	mockRType := model.REC_CNAME
	mockRName := model.DNSRecordName("_test-cn._test")
	mockRec := []model.DNSRecord{{
		Name: "_test-cn._test",
		Type: "CNAME",
		Data: "testing.com",
		TTL:  3600,
	}}

	// add record, read it back
	// also: calls DelRecord if step fails, mb add it as optional
	mockClientAdd := model.NewMockDNSApiClient(t)
	mockClientAdd.EXPECT().AddRecords(mockCtx, mockDom, mockRec).Return(nil).Once()
	mockClientAdd.EXPECT().GetRecords(mockCtx, mockDom, mockRType, mockRName).Return(mockRec, nil)

	// read state
	mockClientImp := model.NewMockDNSApiClient(t)
	mockClientImp.EXPECT().GetRecords(mockCtx, mockDom, mockRType, mockRName).Return(mockRec, nil)

	// read recod, expect mismatch with saved state
	mockClientRef := model.NewMockDNSApiClient(t)
	mockRecRef := slices.Clone(mockRec)
	mockRecRef[0].Data = "changed.com"
	mockClientRef.EXPECT().GetRecords(mockCtx, mockDom, mockRType, mockRName).Return(mockRecRef, nil)

	// read, update, clean up
	// also: must skip update if already ok
	mockClientUpd := model.NewMockDNSApiClient(t)
	rec2set := []model.DNSRecord{{Data: "test.com", TTL: 3600}}
	mockRecUpdated := slices.Clone(mockRec)
	mockRecUpdated[0].Data = "test.com"
	// if using same args + "Once": results could vary on 1st and 2nd call
	// if more than 1 required: .Times(4)
	mockClientUpd.EXPECT().GetRecords(mockCtx, mockDom, mockRType, mockRName).Return(mockRec, nil).Once()
	mockClientUpd.EXPECT().SetRecords(mockCtx, mockDom, mockRType, mockRName, rec2set).Return(nil).Once()
	mockClientUpd.EXPECT().GetRecords(mockCtx, mockDom, mockRType, mockRName).Return(mockRecUpdated, nil).Once()
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
						"_test-cn._test"),
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
			// create + read back
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
						"_test-cn._test"),
					resource.TestCheckResourceAttr(
						resourceName,
						"data",
						"testing.com"),
					CheckApiRecordMach(resourceName, apiClient),
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
	  name   = "_test-cn._test"
	  data   = "%s"
	}`, TEST_DOMAIN, target)
}

func testTXTResourceConfig(target string) string {
	return fmt.Sprintf(`
	provider "godaddy-dns" {}
	locals {testdomain = "%s"}
	resource "godaddy-dns_record" "test-txt" {
	  domain = "${local.testdomain}"
	  type   = "TXT"
	  name   = "test-txt._test"
	  data   = "%s"
	}`, TEST_DOMAIN, target)
}

func CheckApiRecordMach(resourceName string, apiClient *client.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		attrs := s.Modules[0].Resources[resourceName].Primary.Attributes

		apiRecs, err := apiClient.GetRecords(
			context.Background(),
			model.DNSDomain(attrs["domain"]),
			model.DNSRecordType(attrs["type"]),
			model.DNSRecordName(attrs["name"]))
		if err != nil {
			return errors.Wrap(err, "api check client error")
		}
		if len(apiRecs) != 1 {
			return fmt.Errorf("api check: wrong number of results")
		}
		if string(apiRecs[0].Data) != attrs["data"] {
			return fmt.Errorf("api check: data mismatch (%s not %s)", apiRecs[0].Data, attrs["data"])
		}
		return nil
	}
}
