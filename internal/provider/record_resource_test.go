package provider

// run one test: like
// TF_ACC=1 go test -timeout 30s -run='TestAccCnameResource' -v ./internal/provider/
// TF_LOG=info go test -timeout 5s -run='TestUnitCnameResource' -v ./internal/provider/
// sadly, terraform framework hangs when mock calls t.FailNow(), so short timeout
// is essential, especially for automated tests

// also: plan checks in steps: https://developer.hashicorp.com/terraform/plugin/testing/acceptance-tests/plan-checks
//   ConfigPlanChecks: resource.ConfigPlanChecks{
//     PreApply: []plancheck.PlanCheck{
//       plancheck.ExpectEmptyPlan(),  // or coversely ExpectNonEmptyPlan()
//     },
//   },

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"text/template"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/veksh/terraform-provider-godaddy-dns/internal/client"
	"github.com/veksh/terraform-provider-godaddy-dns/internal/model"
)

const TEST_DOMAIN = "veksh.in"

// check for NOOP if delete is performed on resource that is gone already
func TestUnitMXResourceNoDelIfGone(t *testing.T) {
	resourceName := "godaddy-dns_record.test-mx"
	mockCtx := mock.AnythingOfType("*context.valueCtx")
	mockDom := model.DNSDomain(TEST_DOMAIN)
	mockRType := model.REC_MX
	mockRName := model.DNSRecordName("test-mx._test")
	mockRecs := []model.DNSRecord{
		{
			Name:     "test-mx._test",
			Type:     "MX",
			Data:     "mx1.test.com",
			TTL:      3600,
			Priority: 10,
		}, {
			Name:     "test-mx._test",
			Type:     "MX",
			Data:     "mx3.test.com",
			TTL:      3600,
			Priority: 30,
		},
	}

	// add record, read it back
	// also: calls DelRecord if step fails, mb add it as optional
	mockClientAdd := model.NewMockDNSApiClient(t)
	mockClientAdd.EXPECT().AddRecords(mockCtx, mockDom, mockRecs[:1]).Return(nil).Once()
	mockClientAdd.EXPECT().GetRecords(mockCtx, mockDom, mockRType, mockRName).Return(mockRecs, nil)

	// read, skip delete because record is already gone
	mockClientDel := model.NewMockDNSApiClient(t)
	mockRecUpdated := slices.Clone(mockRecs)
	mockRecUpdated[0].Data = "mx2.test.com"
	// need to return it 2 times: 1st for read (refresh), 2nd for delete (keeping recs)
	mockClientDel.EXPECT().GetRecords(mockCtx, mockDom, mockRType, mockRName).Return(mockRecUpdated, nil).Times(2)
	// no need to call set or del: record already gone
	// mockClientDel.EXPECT().DelRecords(mockCtx, mockDom, mockRType, mockRName).Return(nil).Once()

	resource.UnitTest(t, resource.TestCase{
		// ProtoV6ProviderFactories: testProviderFactory,
		Steps: []resource.TestStep{
			// create, read back
			{
				// alt: ConfigFile or ConfigDirectory
				ProtoV6ProviderFactories: mockClientProviderFactory(mockClientAdd),
				Config:                   simpleResourceConfig("MX", "mx1.test.com"),
			},
			// read back, delete (must be noop because already gone)
			{
				ProtoV6ProviderFactories: mockClientProviderFactory(mockClientDel),
				Config:                   simpleResourceConfig("MX", "mx1.test.com"),
				Destroy:                  true,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionDestroy),
					},
				},
			},
		},
	})
}

// check that if remote API state is already ok on plan application and no modification
// is required (e.g. after external change to the resource), no API modification calls
// will be made (although plan will not be empty)
func TestUnitNSResourceNoModIfOk(t *testing.T) {
	// common fixtures
	resourceName := "godaddy-dns_record.test-ns"
	mockCtx := mock.AnythingOfType("*context.valueCtx")
	mockDom := model.DNSDomain(TEST_DOMAIN)
	mockRType := model.REC_NS
	mockRName := model.DNSRecordName("test-ns._test")
	mockRec := []model.DNSRecord{{
		Name: "test-ns._test",
		Type: "NS",
		Data: "ns1.test.com",
		TTL:  3600,
	}}

	// add record, read it back
	// also: calls DelRecord if step fails, mb add it as optional
	mockClientAdd := model.NewMockDNSApiClient(t)
	mockClientAdd.EXPECT().AddRecords(mockCtx, mockDom, mockRec).Return(nil).Once()
	mockClientAdd.EXPECT().GetRecords(mockCtx, mockDom, mockRType, mockRName).Return(mockRec, nil)

	// read, skip update because it is ok already, clean up
	mockClientUpd := model.NewMockDNSApiClient(t)
	mockRecUpdated := slices.Clone(mockRec)
	mockRecUpdated[0].Data = "ns2.test.com"
	// need to return it 2 times: 1st for read (refresh), 2nd for uptate (keeping recs)
	mockClientUpd.EXPECT().GetRecords(mockCtx, mockDom, mockRType, mockRName).Return(mockRecUpdated, nil).Times(2)
	// no need for update: already ok
	// mockClientUpd.EXPECT().SetRecords(mockCtx, mockDom, mockRType, mockRName, rec2set).Return(nil).Once()
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
				Config:                   simpleResourceConfig("NS", "ns1.test.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						resourceName,
						"type",
						"NS"),
					resource.TestCheckResourceAttr(
						resourceName,
						"name",
						"test-ns._test"),
					resource.TestCheckResourceAttr(
						resourceName,
						"data",
						"ns1.test.com"),
				),
			},
			// update, read back, clean up
			{
				ProtoV6ProviderFactories: mockClientProviderFactory(mockClientUpd),
				Config:                   simpleResourceConfig("NS", "ns2.test.com"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						resourceName,
						"data",
						"ns2.test.com"),
				),
			},
		},
	})
}

// test that modifications to TXT record are not affecting another TXT records
// with the same name (by pre-creating one and checking it is ok afterwards)
func TestAccTXTResource(t *testing.T) {

	// client does not complain on empty key/secret if not used
	apiClient, err := client.NewClient(
		GODADDY_API_URL,
		os.Getenv("GODADDY_API_KEY"),
		os.Getenv("GODADDY_API_SECRET"))
	assert.Nil(t, err, "cannot create client")

	resourceName := "godaddy-dns_record.test-txt"
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			rec := []model.DNSRecord{{
				Name: "test-txt._test",
				Type: "TXT",
				Data: "not to be modified",
				TTL:  600,
			}}
			_ = apiClient.AddRecords(context.Background(), TEST_DOMAIN, rec)
			// ignore error: ok to be left over from previous test
		},
		CheckDestroy: func(*terraform.State) error {
			var recs []model.DNSRecord
			recs, err := apiClient.GetRecords(context.Background(),
				TEST_DOMAIN, "TXT", "test-txt._test")
			if err == nil {
				if len(recs) == 0 {
					return fmt.Errorf("too much cleanup: old record did not survive")
				}
				if len(recs) > 1 {
					return fmt.Errorf("too many records left")
				}
				if recs[0].Data != "not to be modified" {
					return fmt.Errorf("unexpectd modification to an old record")
				}
				err = apiClient.DelRecords(context.Background(),
					TEST_DOMAIN, "TXT", "test-txt._test")
			}
			return err
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// create + read back
			{
				// alt: ConfigFile or ConfigDirectory
				Config: simpleResourceConfig("TXT", "test text"),
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
					CheckApiRecordMach(resourceName, apiClient),
				),
			},
			// update + read back
			{
				Config: simpleResourceConfig("TXT", "updated text"),
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

// unit test to check that modifications to TXT record would not affect
// neighbour TXT records with the same name (either already present or
// appeared after first application)
func TestUnitTXTResourceWithAnother(t *testing.T) {
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
				Config:                   simpleResourceConfig("TXT", "test text"),
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
				Config:                   simpleResourceConfig("TXT", "updated text"),
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

// simple unit test for CRUD of TXT record (alone)
func TestUnitTXTResourceAlone(t *testing.T) {
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
				Config:                   simpleResourceConfig("TXT", "test text"),
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
				Config:                   simpleResourceConfig("TXT", "updated text"),
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

// simple unit test for CRUD of CNAME record
func TestUnitCnameResource(t *testing.T) {
	// common fixtures
	resourceName := "godaddy-dns_record.test-cname"
	mockCtx := mock.AnythingOfType("*context.valueCtx")
	mockDom := model.DNSDomain(TEST_DOMAIN)
	mockRType := model.REC_CNAME
	mockRName := model.DNSRecordName("test-cname._test")
	mockRec := []model.DNSRecord{{
		Name: "test-cname._test",
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
				Config:                   simpleResourceConfig("CNAME", "testing.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						resourceName,
						"type",
						"CNAME"),
					resource.TestCheckResourceAttr(
						resourceName,
						"name",
						"test-cname._test"),
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
				Config:                   simpleResourceConfig("CNAME", "test.com"),
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

// simple acceptance test for CRUD of CNAME record
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
		Steps: []resource.TestStep{
			// create + read back
			{
				// alt: ConfigFile or ConfigDirectory
				Config: simpleResourceConfig("CNAME", "testing.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						resourceName,
						"type",
						"CNAME"),
					resource.TestCheckResourceAttr(
						resourceName,
						"name",
						"test-cname._test"),
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
				// ImportStateId: "veksh.in:CNAME:test-cname._testacc:test.com",
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
				Config: simpleResourceConfig("CNAME", "test.com"),
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

func simpleResourceConfig(rectype string, target string) string {
	templateString := `
	provider "godaddy-dns" {}
	resource "godaddy-dns_record" "test-{{ .RecType | lower }}" {
	  domain = "{{ .Domain }}"
	  type   = "{{ .RecType | upper }}"
	  name   = "test-{{ .RecType | lower }}._test"
	  data   = "{{ .RecData }}"
	  {{ if gt .Priority -1 }}
	  priority = {{ .Priority }}
	  {{ end}}
	}`
	funcMap := template.FuncMap{
		"lower": strings.ToLower,
		"upper": strings.ToUpper,
	}
	tmpl, err := template.New("config").Funcs(funcMap).Parse(templateString)
	if err != nil {
		return err.Error()
	}
	var buff strings.Builder
	resConf := struct {
		Domain   string
		RecType  string
		RecData  string
		Priority int
	}{
		TEST_DOMAIN,
		rectype,
		target,
		-1,
	}
	if rectype == "MX" {
		resConf.Priority = 10
	}
	err = tmpl.ExecuteTemplate(&buff, "config", resConf)
	if err != nil {
		return err.Error()
	}
	return buff.String()
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
			return errors.Wrap(err, "result cross-check with API: client error")
		}
		if model.DNSRecordType(attrs["type"]).IsSingleValue() {
			if len(apiRecs) != 1 {
				return fmt.Errorf("result cross-check with API: wrong number of results")
			}
			if string(apiRecs[0].Data) != attrs["data"] {
				return fmt.Errorf("result cross-check with API: data mismatch (%s not %s)", apiRecs[0].Data, attrs["data"])
			}
		} else {
			if len(apiRecs) < 1 {
				return fmt.Errorf("result cross-check with API: no results found")
			}
			for _, rec := range apiRecs {
				if rec.Data == model.DNSRecordData(attrs["data"]) {
					return nil
				}
			}
			return fmt.Errorf("result cross-check with API: none of %d results matched", len(apiRecs))
		}
		return nil
	}
}
