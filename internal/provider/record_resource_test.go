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
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/mock"
	"github.com/veksh/terraform-provider-godaddy-dns/internal/client"
	"github.com/veksh/terraform-provider-godaddy-dns/internal/model"
)

var (
	apiClient, _ = client.NewClient(
		GODADDY_API_URL,
		os.Getenv("GODADDY_API_KEY"),
		os.Getenv("GODADDY_API_SECRET"))

	mCtx = mock.AnythingOfType("*context.valueCtx")
	mDom = model.DNSDomain(TEST_DOMAIN)
)

// simple acceptance test for MX resource, with pre-existing one
func TestAccMXLifecycle(t *testing.T) {
	mType, mName, preExisting, tfResName := makeMockRec(model.REC_MX, "mx2.test.com")
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			_ = apiClient.AddRecords(context.Background(), TEST_DOMAIN, preExisting)
			// ignore error: ok to be left over from previous test
		},
		CheckDestroy: func(*terraform.State) error {
			var recs []model.DNSRecord
			recs, err := apiClient.GetRecords(context.Background(),
				TEST_DOMAIN, mType, mName)
			if err == nil {
				if len(recs) == 0 {
					return fmt.Errorf("too much cleanup: old record did not survive")
				}
				if len(recs) > 1 {
					return fmt.Errorf("too many records left")
				}
				if recs[0] != preExisting[0] {
					return fmt.Errorf("unexpectd modification to an old record: want %v got %v", preExisting[0], recs[0])
				}
				err = apiClient.DelRecords(context.Background(),
					TEST_DOMAIN, mType, mName)
			}
			return err
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// create + read back
			{
				Config: simpleResourceConfig("MX", "mx1.test.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						tfResName,
						"data",
						"mx1.test.com"),
					CheckApiRecordMach(tfResName, apiClient),
				),
			},
			// update + read back, then destroy
			{
				Config: simpleResourceConfig("MX", "mx1-new.test.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						tfResName,
						"data",
						"mx1-new.test.com"),
					CheckApiRecordMach(tfResName, apiClient),
				),
			},
		},
	})
}

// simple MX resource lifecycle
func TestUnitMXLifecycle(t *testing.T) {
	mType, mName, _, _ := makeMockRec(model.REC_MX, "unused")
	mRecs := []model.DNSRecord{
		{
			Name:     mName,
			Type:     mType,
			Data:     "mx3.test.com",
			TTL:      3600,
			Priority: 30,
		}, {
			Name:     mName,
			Type:     mType,
			Data:     "mx1.test.com",
			TTL:      3600,
			Priority: 10,
		},
	}
	mUpdates := []model.DNSRecord{
		{
			Data:     "mx3.test.com",
			TTL:      3600,
			Priority: 30,
		}, {
			Data:     "mx2.test.com",
			TTL:      3600,
			Priority: 10,
		},
	}

	// add record, read it back
	mockClientAdd := model.NewMockDNSApiClient(t)
	mockClientAdd.EXPECT().AddRecords(mCtx, mDom, mRecs[1:2]).Return(nil).Once()
	mockClientAdd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecs, nil)

	// read, update, then delete
	mockClientUpd := model.NewMockDNSApiClient(t)
	mRecsUpdated := slices.Clone(mRecs)
	mRecsUpdated[1].Data = "mx2.test.com"
	// read + update
	mockClientUpd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecs, nil).Twice()
	mockClientUpd.EXPECT().SetRecords(mCtx, mDom, mType, mName, mUpdates).Return(nil).Once()
	// cleanup: delete by setting it back
	mockClientUpd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecsUpdated, nil).Twice()
	mockClientUpd.EXPECT().SetRecords(mCtx, mDom, mType, mName, mUpdates[:1]).Return(nil).Once()

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
				ProtoV6ProviderFactories: mockClientProviderFactory(mockClientUpd),
				Config:                   simpleResourceConfig("MX", "mx2.test.com"),
			},
		},
	})
}

// check for NOOP if delete is performed on resource that is gone already
func TestUnitMXNoopDelIfGone(t *testing.T) {
	mType, mName, _, tfResName := makeMockRec(model.REC_MX, "unused")
	mRecs := []model.DNSRecord{
		{
			Name:     "test-mx._test",
			Type:     mType,
			Data:     "mx1.test.com",
			TTL:      3600,
			Priority: 10,
		}, {
			Name:     "test-mx._test",
			Type:     mType,
			Data:     "mx3.test.com",
			TTL:      3600,
			Priority: 30,
		},
	}

	// add record, read it back
	mClientAdd := model.NewMockDNSApiClient(t)
	mClientAdd.EXPECT().AddRecords(mCtx, mDom, mRecs[:1]).Return(nil).Once()
	mClientAdd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecs, nil)

	// read, skip delete because record is already gone
	mClientDel := model.NewMockDNSApiClient(t)
	mRecsUpdated := slices.Clone(mRecs)
	mRecsUpdated[0].Data = "mx2.test.com"
	// need to return it 2 times: 1st for read (refresh), 2nd for delete (keeping recs)
	mClientDel.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecsUpdated, nil).Times(2)
	// no need to call set or del: record already gone
	// mockClientDel.EXPECT().DelRecords(mockCtx, mockDom, mockRType, mockRName).Return(nil).Once()

	resource.UnitTest(t, resource.TestCase{
		// ProtoV6ProviderFactories: testProviderFactory,
		Steps: []resource.TestStep{
			// create, read back
			{
				// alt: ConfigFile or ConfigDirectory
				ProtoV6ProviderFactories: mockClientProviderFactory(mClientAdd),
				Config:                   simpleResourceConfig("MX", "mx1.test.com"),
			},
			// read back, delete (must be noop because already gone)
			{
				ProtoV6ProviderFactories: mockClientProviderFactory(mClientDel),
				Config:                   simpleResourceConfig("MX", "mx1.test.com"),
				Destroy:                  true,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(tfResName, plancheck.ResourceActionDestroy),
					},
				},
			},
		},
	})
}

// check that if remote API state is already ok on plan application and no modification
// is required (e.g. after external change to the resource), no API modification calls
// will be made (although plan will not be empty)
func TestUnitNSNoopModIfOk(t *testing.T) {
	mType, mName, mRecs, tfResName := makeMockRec(model.REC_NS, "ns1.test.com")

	// add record, read it back
	// also: calls DelRecord if step fails, mb add it as optional
	mClientAdd := model.NewMockDNSApiClient(t)
	mClientAdd.EXPECT().AddRecords(mCtx, mDom, mRecs).Return(nil).Once()
	mClientAdd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecs, nil)

	// read, skip update because it is ok already, clean up
	mClientUpd := model.NewMockDNSApiClient(t)
	mRecsUpdated := slices.Clone(mRecs)
	mRecsUpdated[0].Data = "ns2.test.com"
	// need to return it 2 times: 1st for read (refresh), 2nd for uptate (keeping recs)
	mClientUpd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecsUpdated, nil).Times(2)
	// no need for update: already ok
	// mockClientUpd.EXPECT().SetRecords(mockCtx, mockDom, mockRType, mockRName, rec2set).Return(nil).Once()
	// same thing with delete
	mClientUpd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecsUpdated, nil).Times(2)
	mClientUpd.EXPECT().DelRecords(mCtx, mDom, mType, mName).Return(nil).Once()

	resource.UnitTest(t, resource.TestCase{
		// ProtoV6ProviderFactories: testProviderFactory,
		Steps: []resource.TestStep{
			// create, read back
			{
				// alt: ConfigFile or ConfigDirectory
				ProtoV6ProviderFactories: mockClientProviderFactory(mClientAdd),
				Config:                   simpleResourceConfig("NS", "ns1.test.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						tfResName,
						"data",
						"ns1.test.com"),
				),
			},
			// update, read back, clean up
			{
				ProtoV6ProviderFactories: mockClientProviderFactory(mClientUpd),
				Config:                   simpleResourceConfig("NS", "ns2.test.com"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(tfResName, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						tfResName,
						"data",
						"ns2.test.com"),
				),
			},
		},
	})
}

// test that modifications to TXT record are not affecting another TXT records
// with the same name (by pre-creating one and checking it is ok afterwards)
func TestAccTXTLifecycle(t *testing.T) {
	mType, mName, preExisting, tfResName := makeMockRec(model.REC_TXT, "not to be modified")
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			_ = apiClient.AddRecords(context.Background(), TEST_DOMAIN, preExisting)
			// ignore error: ok to be left over from previous test
		},
		CheckDestroy: func(*terraform.State) error {
			var recs []model.DNSRecord
			recs, err := apiClient.GetRecords(context.Background(),
				TEST_DOMAIN, mType, mName)
			if err == nil {
				if len(recs) == 0 {
					return fmt.Errorf("too much cleanup: old record did not survive")
				}
				if len(recs) > 1 {
					return fmt.Errorf("too many records left")
				}
				if recs[0] != preExisting[0] {
					return fmt.Errorf("unexpectd modification to an old record")
				}
				err = apiClient.DelRecords(context.Background(),
					TEST_DOMAIN, mType, mName)
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
						tfResName,
						"data",
						"test text"),
					CheckApiRecordMach(tfResName, apiClient),
				),
			},
			// update + read back
			{
				Config: simpleResourceConfig("TXT", "updated text"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						tfResName,
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
func TestUnitTXTWithAnother(t *testing.T) {
	mType, mName, mRecs, tfResName := makeMockRec(model.REC_TXT, "test text")
	mRec := mRecs[0]
	mRecAnother := model.DNSRecord{
		Name: mName,
		Type: mType,
		Data: "do not modify",
		TTL:  600}
	mRecs = append(mRecs, mRecAnother)
	mRecYetAnother := model.DNSRecord{
		Name: mName,
		Type: mType,
		Data: "also appears",
		TTL:  7200}

	// add record, read it back
	// also: calls DelRecord if step fails, mb add it as optional
	mClientAdd := model.NewMockDNSApiClient(t)
	mClientAdd.EXPECT().AddRecords(mCtx, mDom, []model.DNSRecord{mRec}).Return(nil).Once()
	mClientAdd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecs, nil)

	// read state
	mClientImp := model.NewMockDNSApiClient(t)
	mClientImp.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecs, nil)

	// read recod (similate changes, so rec not found), expect mismatch with saved state
	// also: tries to destroy refreshed RR if last in pipeline; mostly ok
	mClientRef := model.NewMockDNSApiClient(t)
	mRecsRefresh := slices.Clone(mRecs)
	mRecsRefresh[0].Data = "changed text"
	mClientRef.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecsRefresh, nil)

	// read (simulate another record added, and ours still present), update
	// final step: clean up (not delete but set with 2 remaining records)
	mClientUpd := model.NewMockDNSApiClient(t)
	rec2set := []model.DNSRecord{{Data: "do not modify", TTL: 600}, {Data: "updated text", TTL: 3600}}
	mRecsUpdated := slices.Clone(mRecs)
	mRecsUpdated[0].Data = "updated text"
	mRecsUpdated = append(mRecsUpdated, mRecYetAnother)
	recs2keep := []model.DNSRecord{{Data: "do not modify", TTL: 600}, {Data: "also appears", TTL: 7200}}
	// 2 gets: 1st for read/refresh, 2nd for uptate/find recs to keep
	mClientUpd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecs, nil).Twice()
	mClientUpd.EXPECT().SetRecords(mCtx, mDom, mType, mName, rec2set).Return(nil).Once()
	// same thing with delete: refresh, enumerate recs to keep
	mClientUpd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecsUpdated, nil).Twice()
	mClientUpd.EXPECT().SetRecords(mCtx, mDom, mType, mName, recs2keep).Return(nil).Once()

	resource.UnitTest(t, resource.TestCase{
		// ProtoV6ProviderFactories: testProviderFactory,
		Steps: []resource.TestStep{
			// create, read back
			{
				// alt: ConfigFile or ConfigDirectory
				ProtoV6ProviderFactories: mockClientProviderFactory(mClientAdd),
				Config:                   simpleResourceConfig("TXT", "test text"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						tfResName,
						"data",
						"test text"),
				),
			},
			// read, compare with saved, should produce no plan
			{
				ProtoV6ProviderFactories: mockClientProviderFactory(mClientImp),
				ResourceName:             tfResName,
				ImportState:              true,
				ImportStateVerify:        true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					attrs := s.Modules[0].Resources[tfResName].Primary.Attributes
					return fmt.Sprintf("%s:%s:%s:%s",
						attrs["domain"], attrs["type"], attrs["name"], attrs["data"]), nil
				},
				ImportStateVerifyIdentifierAttribute: "name",
			},
			// read, compare with saved, should produce update plan
			{
				ProtoV6ProviderFactories: mockClientProviderFactory(mClientRef),
				ResourceName:             tfResName,
				RefreshState:             true,
				ExpectNonEmptyPlan:       true,
			},
			// update, read back, clean up (keeping others)
			{
				ProtoV6ProviderFactories: mockClientProviderFactory(mClientUpd),
				Config:                   simpleResourceConfig("TXT", "updated text"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						tfResName,
						"data",
						"updated text"),
				),
			},
		},
	})
}

// simple unit test for CRUD of TXT record (alone)
func TestUnitTXTLifecycle(t *testing.T) {
	mType, mName, mRecs, tfResName := makeMockRec(model.REC_TXT, "test text")

	// add record, read it back
	// also: calls DelRecord if step fails, mb add it as optional
	mClientAdd := model.NewMockDNSApiClient(t)
	mClientAdd.EXPECT().AddRecords(mCtx, mDom, mRecs).Return(nil).Once()
	mClientAdd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecs, nil)

	// read state
	mClientImp := model.NewMockDNSApiClient(t)
	mClientImp.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecs, nil)

	// read recod, expect mismatch with saved state
	// also: tries to destroy refreshed RR if last in pipeline; mostly ok
	mClientRef := model.NewMockDNSApiClient(t)
	mRecsRefresh := slices.Clone(mRecs)
	mRecsRefresh[0].Data = "changed text"
	mClientRef.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecsRefresh, nil)

	// read, update, clean up
	// also: must skip update if already ok
	mClientUpd := model.NewMockDNSApiClient(t)
	rec2set := []model.DNSRecord{{Data: "updated text", TTL: 3600}}
	mRecsUpdated := slices.Clone(mRecs)
	mRecsUpdated[0].Data = "updated text"
	// need to return it 2 times: 1st for read (refresh), 2nd for uptate (keeping recs)
	mClientUpd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecs, nil).Times(2)
	mClientUpd.EXPECT().SetRecords(mCtx, mDom, mType, mName, rec2set).Return(nil).Once()
	// same thing with delete
	mClientUpd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecsUpdated, nil).Times(2)
	mClientUpd.EXPECT().DelRecords(mCtx, mDom, mType, mName).Return(nil).Once()

	resource.UnitTest(t, resource.TestCase{
		// ProtoV6ProviderFactories: testProviderFactory,
		Steps: []resource.TestStep{
			// create, read back
			{
				// alt: ConfigFile or ConfigDirectory
				ProtoV6ProviderFactories: mockClientProviderFactory(mClientAdd),
				Config:                   simpleResourceConfig("TXT", "test text"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						tfResName,
						"data",
						"test text"),
				),
			},
			// read, compare with saved, should produce no plan
			{
				ProtoV6ProviderFactories: mockClientProviderFactory(mClientImp),
				ResourceName:             tfResName,
				ImportState:              true,
				ImportStateVerify:        true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					attrs := s.Modules[0].Resources[tfResName].Primary.Attributes
					return fmt.Sprintf("%s:%s:%s:%s",
						attrs["domain"], attrs["type"], attrs["name"], attrs["data"]), nil
				},
				ImportStateVerifyIdentifierAttribute: "name",
			},
			// read, compare with saved, should produce update plan
			{
				ProtoV6ProviderFactories: mockClientProviderFactory(mClientRef),
				ResourceName:             tfResName,
				RefreshState:             true,
				ExpectNonEmptyPlan:       true,
			},
			// update, read back, clean up
			{
				ProtoV6ProviderFactories: mockClientProviderFactory(mClientUpd),
				Config:                   simpleResourceConfig("TXT", "updated text"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						tfResName,
						"data",
						"updated text"),
				),
			},
		},
	})
}

// simple unit test for CRUD of CNAME record
func TestUnitCnameLifecycle(t *testing.T) {
	mType, mName, mRecs, tfResName := makeMockRec(model.REC_CNAME, "testing.com")

	// add record, read it back
	// also: calls DelRecord if step fails, mb add it as optional
	mClientAdd := model.NewMockDNSApiClient(t)
	mClientAdd.EXPECT().AddRecords(mCtx, mDom, mRecs).Return(nil).Once()
	mClientAdd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecs, nil)

	// read state
	mClientImp := model.NewMockDNSApiClient(t)
	mClientImp.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecs, nil)

	// read recod, expect mismatch with saved state
	mClientRef := model.NewMockDNSApiClient(t)
	mockRecRef := slices.Clone(mRecs)
	mockRecRef[0].Data = "changed.com"
	mClientRef.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mockRecRef, nil)

	// read, update, clean up
	// also: must skip update if already ok
	mClientUpd := model.NewMockDNSApiClient(t)
	rec2set := []model.DNSRecord{{Data: "test.com", TTL: 3600}}
	mRecsUpdated := slices.Clone(mRecs)
	mRecsUpdated[0].Data = "test.com"
	// if using same args + "Once": results could vary on 1st and 2nd call
	// if more than 1 required: .Times(4)
	mClientUpd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecs, nil).Once()
	mClientUpd.EXPECT().SetRecords(mCtx, mDom, mType, mName, rec2set).Return(nil).Once()
	mClientUpd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecsUpdated, nil).Once()
	mClientUpd.EXPECT().DelRecords(mCtx, mDom, mType, mName).Return(nil).Once()

	resource.UnitTest(t, resource.TestCase{
		// ProtoV6ProviderFactories: testProviderFactory,
		Steps: []resource.TestStep{
			// create, read back
			{
				// alt: ConfigFile or ConfigDirectory
				ProtoV6ProviderFactories: mockClientProviderFactory(mClientAdd),
				Config:                   simpleResourceConfig("CNAME", "testing.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						tfResName,
						"data",
						"testing.com"),
				),
			},
			// read, compare with saved, should produce no plan
			{
				ProtoV6ProviderFactories: mockClientProviderFactory(mClientImp),
				ResourceName:             tfResName,
				ImportState:              true,
				ImportStateVerify:        true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					attrs := s.Modules[0].Resources[tfResName].Primary.Attributes
					return fmt.Sprintf("%s:%s:%s:%s",
						attrs["domain"], attrs["type"], attrs["name"], attrs["data"]), nil
				},
				ImportStateVerifyIdentifierAttribute: "name",
			},
			// read, compare with saved, should produce update plan
			{
				ProtoV6ProviderFactories: mockClientProviderFactory(mClientRef),
				ResourceName:             tfResName,
				RefreshState:             true,
				ExpectNonEmptyPlan:       true,
			},
			// update, read back
			{
				ProtoV6ProviderFactories: mockClientProviderFactory(mClientUpd),
				Config:                   simpleResourceConfig("CNAME", "test.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						tfResName,
						"data",
						"test.com"),
				),
			},
		},
	})
}

// simple acceptance test for CRUD of CNAME record
func TestAccCnameLifecycle(t *testing.T) {
	tfResName := "godaddy-dns_record.test-cname"
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
						tfResName,
						"data",
						"testing.com"),
					CheckApiRecordMach(tfResName, apiClient),
				),
			},
			// import state
			{
				ResourceName:      tfResName,
				ImportState:       true,
				ImportStateVerify: true,
				// ImportStateId: "veksh.in:CNAME:test-cname._testacc:test.com",
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					attrs := s.Modules[0].Resources[tfResName].Primary.Attributes
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
						tfResName,
						"data",
						"test.com"),
				),
			},
		},
	})
}
