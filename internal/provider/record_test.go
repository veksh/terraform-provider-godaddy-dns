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

// two A resources + 1 pre-existing, all with the same hame
func TestUnitALifecycle(t *testing.T) {
	// this will be found as pre-existing
	mRecsPre := makeTestRecSet(model.REC_A, []model.DNSRecordData{"1.1.1.1"})
	// records to add
	mRecsToAdd := makeTestRecSet(model.REC_A, []model.DNSRecordData{"2.2.2.2", "3.3.3.3"})
	// after adding 1/2 + kept pre
	mRecsTgt1 := makeTestRecSet(model.REC_A, []model.DNSRecordData{"1.1.1.1", "2.2.2.2"})
	// mRecsTgt2 := makeTestRecSet(model.REC_A, []model.DNSRecordData{"1.1.1.1", "3.3.3.3"})
	// after adding 2/2 + kept pre
	mRecsTgt := makeTestRecSet(model.REC_A, []model.DNSRecordData{"1.1.1.1", "2.2.2.2", "3.3.3.3"})

	// 2nd step: update
	mRecsToUpd := makeTestRecSet(model.REC_A, []model.DNSRecordData{"2.2.2.2", "4.4.4.4"})
	mRecsToUpdTgt := makeTestRecSet(model.REC_A, []model.DNSRecordData{"1.1.1.1", "2.2.2.2", "4.4.4.4"})
	mRecsTgt4 := makeTestRecSet(model.REC_A, []model.DNSRecordData{"1.1.1.1", "4.4.4.4"})

	mType, mName := model.REC_A, mRecsPre.DNSRecName

	// add records for .2 and .3, read it back (with pre .1)
	mockClientAdd := model.NewMockDNSApiClient(t)
	mockClientAdd.EXPECT().AddRecords(mCtx, mDom, mRecsToAdd.Records[0:1]).Return(nil).Once()
	mockClientAdd.EXPECT().AddRecords(mCtx, mDom, mRecsToAdd.Records[1:2]).Return(nil).Once()
	mockClientAdd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecsTgt.Records, nil).Twice()

	// destroy after 1st step: cleanup right after 1st step; really not deterministic:
	// mockClientAdd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecsTgt.Records, nil).Once()
	// mockClientAdd.EXPECT().SetRecords(mCtx, mDom, mType, mName, mRecsTgt1.UpdRecords).Return(nil).Once()
	// mockClientAdd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecsTgt1.Records, nil).Once()
	// mockClientAdd.EXPECT().SetRecords(mCtx, mDom, mType, mName, mRecsPre.UpdRecords).Return(nil).Once()

	// destroy after 1st step: a bit absurd, but at least stable :)
	// mockClientAdd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecsTgt.Records, nil).Twice()
	// mockClientAdd.EXPECT().SetRecords(mCtx, mDom, mType, mName, mRecsTgt1.UpdRecords).Return(nil).Once()
	// mockClientAdd.EXPECT().SetRecords(mCtx, mDom, mType, mName, mRecsTgt2.UpdRecords).Return(nil).Once()

	// read, update, then delete for clean-up
	mockClientUpd := model.NewMockDNSApiClient(t)
	mockClientUpd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecsTgt.Records, nil).Times(3)
	mockClientUpd.EXPECT().SetRecords(mCtx, mDom, mType, mName, mRecsToUpdTgt.UpdRecords).Return(nil).Once()
	// again, clean-up order is not deterministic, so lets return same results to both and see them keeping pre
	// better way would be "3 recs -> del 3rd -> 2 recs -> del 2nd -> only pre-existing left"
	// the problem is that clean-up is going in parallel, so it is hard to return a proper results,
	// and there is no way to express these dependencies in mock
	mockClientUpd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecsToUpdTgt.Records, nil).Times(4)
	mockClientUpd.EXPECT().SetRecords(mCtx, mDom, mType, mName, mRecsTgt1.UpdRecords).Return(nil).Once()
	mockClientUpd.EXPECT().SetRecords(mCtx, mDom, mType, mName, mRecsTgt4.UpdRecords).Return(nil).Once()

	resource.UnitTest(t, resource.TestCase{
		// ProtoV6ProviderFactories: testProviderFactory,
		Steps: []resource.TestStep{
			// create, read back
			{
				ProtoV6ProviderFactories: mockClientProviderFactory(mockClientAdd),
				Config:                   mRecsToAdd.TFConfig,
			},
			// read back, change, delete (must be noop because already gone)
			{
				ProtoV6ProviderFactories: mockClientProviderFactory(mockClientUpd),
				Config:                   mRecsToUpd.TFConfig,
			},
		},
	})
}

// simple acceptance test for MX resource, with pre-existing one
func TestAccMXLifecycle(t *testing.T) {
	mData := model.DNSRecordData("mx1.test.com")
	mDataOther := model.DNSRecordData("mx2.test.com")
	mDataChanged := model.DNSRecordData("mx1-new.test.com")
	mType, mName, preExisting, tfResName := makeMockRec(model.REC_MX, mDataOther)
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
				Config: simpleResourceConfig(model.REC_MX, mData),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						tfResName, "data", string(mData)),
					CheckApiRecordMach(tfResName, apiClient),
				),
			},
			// update + read back, then destroy
			{
				Config: simpleResourceConfig(model.REC_MX, mDataChanged),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						tfResName, "data", string(mDataChanged)),
					CheckApiRecordMach(tfResName, apiClient),
				),
			},
		},
	})
}

// simple MX resource lifecycle
func TestUnitMXLifecycle(t *testing.T) {
	mType, mName, _, _ := makeMockRec(model.REC_MX, "unused")
	mData := model.DNSRecordData("mx1.test.com")
	mDataChanged := model.DNSRecordData("mx2.test.com")
	mDataOther := model.DNSRecordData("mx3.test.com")
	mRecs := []model.DNSRecord{
		{
			Name:     mName,
			Type:     mType,
			Data:     mDataOther,
			TTL:      3600,
			Priority: 30,
		}, {
			Name:     mName,
			Type:     mType,
			Data:     mData,
			TTL:      3600,
			Priority: 10,
		},
	}
	mUpdates := []model.DNSUpdateRecord{
		{
			Data:     mDataOther,
			TTL:      3600,
			Priority: 30,
		}, {
			Data:     mDataChanged,
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
	mRecsUpdated[1].Data = mDataChanged
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
				ProtoV6ProviderFactories: mockClientProviderFactory(mockClientAdd),
				Config:                   simpleResourceConfig(model.REC_MX, mData),
			},
			// read back, delete (must be noop because already gone)
			{
				ProtoV6ProviderFactories: mockClientProviderFactory(mockClientUpd),
				Config:                   simpleResourceConfig(model.REC_MX, mDataChanged),
			},
		},
	})
}

// check for NOOP if delete is performed on resource that is gone already
func TestUnitMXNoopDelIfGone(t *testing.T) {
	mData := model.DNSRecordData("mx1.test.com")
	mDataChanged := model.DNSRecordData("mx2.test.com")
	mDataOther := model.DNSRecordData("mx3.test.com")
	mType, mName, mRecs, tfResName := makeMockRec(model.REC_MX, mData)
	mRecs = append(mRecs, model.DNSRecord{
		Name:     mName,
		Type:     mType,
		Data:     mDataOther,
		TTL:      3600,
		Priority: 30})

	// add record, read it back
	mClientAdd := model.NewMockDNSApiClient(t)
	mClientAdd.EXPECT().AddRecords(mCtx, mDom, mRecs[:1]).Return(nil).Once()
	mClientAdd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecs, nil)

	// read, skip delete because record is already gone
	mClientDel := model.NewMockDNSApiClient(t)
	mRecsUpdated := slices.Clone(mRecs)
	mRecsUpdated[0].Data = mDataChanged
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
				Config:                   simpleResourceConfig(model.REC_MX, mData),
			},
			// read back, delete (must be noop because already gone)
			{
				ProtoV6ProviderFactories: mockClientProviderFactory(mClientDel),
				Config:                   simpleResourceConfig(model.REC_MX, mData),
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

// check what will happen on update if remote record is already modified (externally)
// currently this will result in "create" for new record and, most probably, an error
// from API ("create" op does not currently check for duplicates); lets fix it here
// as an expected outcome
func TestUnitNSExtMod(t *testing.T) {
	mData := model.DNSRecordData("ns1.test.com")
	mDataChanged := model.DNSRecordData("ns2.test.com")
	mType, mName, mRecs, tfResName := makeMockRec(model.REC_NS, mData)

	// add record, read it back
	// also: calls DelRecord if step fails, mb add it as optional
	mClientAdd := model.NewMockDNSApiClient(t)
	mClientAdd.EXPECT().AddRecords(mCtx, mDom, mRecs).Return(nil).Once()
	mClientAdd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecs, nil).Once()

	// read updated value with updated config, skip change because it is ok already, clean up
	mClientUpd := model.NewMockDNSApiClient(t)
	mRecsUpdated := slices.Clone(mRecs)
	mRecsUpdated[0].Data = mDataChanged

	// // new way: realistic expectations
	// read records on refres, see old one is gone
	mClientUpd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecsUpdated, nil).Once()
	// add new; actually this would give error if using real API (dup errors)
	mClientUpd.EXPECT().AddRecords(mCtx, mDom, mRecsUpdated).Return(nil).Once()
	// cleanup, delete
	mClientUpd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecsUpdated, nil).Times(2)
	mClientUpd.EXPECT().DelRecords(mCtx, mDom, mType, mName).Return(nil).Once()

	resource.UnitTest(t, resource.TestCase{
		// ProtoV6ProviderFactories: testProviderFactory,
		Steps: []resource.TestStep{
			// create, read back
			{
				// alt: ConfigFile or ConfigDirectory
				ProtoV6ProviderFactories: mockClientProviderFactory(mClientAdd),
				Config:                   simpleResourceConfig(model.REC_NS, mData),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(tfResName, "data", string(mData)),
				),
			},
			// update, read back, clean up
			{
				ProtoV6ProviderFactories: mockClientProviderFactory(mClientUpd),
				Config:                   simpleResourceConfig(model.REC_NS, mDataChanged),
				// ConfigPlanChecks: resource.ConfigPlanChecks{
				// 	PreApply: []plancheck.PlanCheck{
				// 		plancheck.ExpectResourceAction(tfResName, plancheck.ResourceActionUpdate),
				// 	},
				// },
				// Check: resource.ComposeAggregateTestCheckFunc(
				// 	resource.TestCheckResourceAttr(
				// 		tfResName, "data", string(mDataChanged)),
				// ),
			},
		},
	})
}

// test that modifications to TXT record are not affecting another TXT records
// with the same name (by pre-creating one and checking it is ok afterwards)
func TestAccTXTLifecycle(t *testing.T) {
	mData := model.DNSRecordData("test text")
	mDataChanged := model.DNSRecordData("updated text" + IMPORT_SEP + " separator")
	mDataOther := model.DNSRecordData("not to be modified")
	mType, mName, preExisting, tfResName := makeMockRec(model.REC_TXT, mDataOther)
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
				Config: simpleResourceConfig(model.REC_TXT, mData),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(tfResName, "data", string(mData)),
					CheckApiRecordMach(tfResName, apiClient),
				),
			},
			// update + read back
			{
				Config: simpleResourceConfig(model.REC_TXT, mDataChanged),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						tfResName,
						"data",
						string(mDataChanged)),
				),
			},
		},
	})
}

// unit test to check that modifications to TXT record would not affect
// neighbour TXT records with the same name (either already present or
// appeared after first application)
func TestUnitTXTWithAnother(t *testing.T) {

	mType := model.REC_TXT
	mDataPre := model.DNSRecordData("pre-existing text")
	mRecsPre := makeTestRecSet(mType, []model.DNSRecordData{mDataPre})

	mData := model.DNSRecordData("test text")
	mRecs := makeTestRecSet(mType, []model.DNSRecordData{mData})
	mRecsPlusPre := makeTestRecSet(mType, []model.DNSRecordData{mDataPre, mData})
	mName := mRecs.DNSRecName

	mDataChanged := model.DNSRecordData("changed text" + IMPORT_SEP + " here")
	// mRecsChange := makeTestRecSet(mType, []model.DNSRecordData{mDataChanged})
	mRecsChanged := makeTestRecSet(mType, []model.DNSRecordData{mDataChanged})
	// order is significant
	mRecsChangedPlusPre := makeTestRecSet(mType, []model.DNSRecordData{mDataPre, mDataChanged})

	// add record, read it back
	// also: calls DelRecord if step fails, mb add it as optional
	mClientAdd := model.NewMockDNSApiClient(t)
	mClientAdd.EXPECT().AddRecords(mCtx, mDom, mRecs.Records).Return(nil).Once()
	mClientAdd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecsPlusPre.Records, nil).Once()

	// // read state
	// mClientImp := model.NewMockDNSApiClient(t)
	// mClientImp.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecs.Records, nil).Once()

	// read back old + pre, update, cleanup
	mClientUpd := model.NewMockDNSApiClient(t)
	// read, update (read-keep + set)
	mClientUpd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecsPlusPre.Records, nil).Twice()
	mClientUpd.EXPECT().SetRecords(mCtx, mDom, mType, mName, mRecsChangedPlusPre.UpdRecords).Return(nil).Once()
	// same thing with delete: read, read-keep, set
	mClientUpd.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecsChangedPlusPre.Records, nil).Twice()
	mClientUpd.EXPECT().SetRecords(mCtx, mDom, mType, mName, mRecsPre.UpdRecords).Return(nil).Once()

	resource.UnitTest(t, resource.TestCase{
		// ProtoV6ProviderFactories: testProviderFactory,
		Steps: []resource.TestStep{
			// create, read back
			{
				// alt: ConfigFile or ConfigDirectory
				ProtoV6ProviderFactories: mockClientProviderFactory(mClientAdd),
				Config:                   mRecs.TFConfig,
				// Check: resource.ComposeAggregateTestCheckFunc(
				// 	resource.TestCheckResourceAttr(tfResName, "data", string(mData)),
				// ),
			},
			// read, compare with saved, should produce no plan
			// {
			// 	ProtoV6ProviderFactories: mockClientProviderFactory(mClientImp),
			// 	ResourceName:             tfResName,
			// 	ImportState:              true,
			// 	ImportStateVerify:        true,
			// 	ImportStateIdFunc: func(s *terraform.State) (string, error) {
			// 		attrs := s.Modules[0].Resources[tfResName].Primary.Attributes
			// 		return fmt.Sprintf("%s:%s:%s:%s",
			// 			attrs["domain"], attrs["type"], attrs["name"], attrs["data"]), nil
			// 	},
			// 	ImportStateVerifyIdentifierAttribute: "name",
			// },
			// update, read back, clean up (keeping others)
			{
				ProtoV6ProviderFactories: mockClientProviderFactory(mClientUpd),
				Config:                   mRecsChanged.TFConfig,
				// not working with mRecs.TFResName + "[0]"
				// Check: resource.ComposeAggregateTestCheckFunc(
				// 	resource.TestCheckResourceAttr(mRecs.TFResName, "data", string(mDataChanged)),
				// ),
			},
		},
	})
}

// simple unit test for CRUD of TXT record (alone)
func TestUnitTXTLifecycle(t *testing.T) {
	mData := model.DNSRecordData("test text")
	mDataOther := model.DNSRecordData("do not modify")
	mDataChanged := model.DNSRecordData("changed text")
	mType, mName, mRecs, tfResName := makeMockRec(model.REC_TXT, mData)

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
	mRecsRefresh[0].Data = mDataOther
	mClientRef.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mRecsRefresh, nil)

	// read, update, clean up
	// also: must skip update if already ok
	mClientUpd := model.NewMockDNSApiClient(t)
	rec2set := []model.DNSUpdateRecord{{Data: mDataChanged, TTL: 3600}}
	mRecsUpdated := slices.Clone(mRecs)
	mRecsUpdated[0].Data = mDataChanged
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
				Config:                   simpleResourceConfig(model.REC_TXT, mData),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(tfResName, "data", string(mData)),
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
				Config:                   simpleResourceConfig(model.REC_TXT, mDataChanged),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(tfResName, "data", string(mDataChanged)),
				),
			},
		},
	})
}

// simple unit test for CRUD of CNAME record
func TestUnitCnameLifecycle(t *testing.T) {
	mData := model.DNSRecordData("testing.com")
	mDataOther := model.DNSRecordData("changed.com")
	mDataChanged := model.DNSRecordData("test.com")
	mType, mName, mRecs, tfResName := makeMockRec(model.REC_CNAME, mData)

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
	mockRecRef[0].Data = mDataOther
	mClientRef.EXPECT().GetRecords(mCtx, mDom, mType, mName).Return(mockRecRef, nil)

	// read, update, clean up
	// also: must skip update if already ok
	mClientUpd := model.NewMockDNSApiClient(t)
	rec2set := []model.DNSUpdateRecord{{Data: mDataChanged, TTL: 3600}}
	mRecsUpdated := slices.Clone(mRecs)
	mRecsUpdated[0].Data = mDataChanged
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
				Config:                   simpleResourceConfig(model.REC_CNAME, mData),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(tfResName, "data", string(mData)),
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
				Config:                   simpleResourceConfig(model.REC_CNAME, mDataChanged),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(tfResName, "data", string(mDataChanged)),
				),
			},
		},
	})
}

// simple acceptance test for CRUD of CNAME record
func TestAccCnameLifecycle(t *testing.T) {
	mData := model.DNSRecordData("testing.com")
	mDataChanged := model.DNSRecordData("test.com")
	tfResName := "godaddy-dns_record.test-cname"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// create + read back
			{
				// alt: ConfigFile or ConfigDirectory
				Config: simpleResourceConfig(model.REC_CNAME, mData),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(tfResName, "data", string(mData)),
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
				Config: simpleResourceConfig(model.REC_CNAME, mDataChanged),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(tfResName, "data", string(mDataChanged)),
				),
			},
		},
	})
}
