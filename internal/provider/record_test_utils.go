package provider

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"text/template"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/veksh/terraform-provider-godaddy-dns/internal/client"
	"github.com/veksh/terraform-provider-godaddy-dns/internal/model"
)

const TEST_DOMAIN = "veksh.in"

// common pre-checks for all acceptance tests: for now, check env secrets presence
func testAccPreCheck(t *testing.T) {
	if os.Getenv("GODADDY_API_KEY") == "" || os.Getenv("GODADDY_API_SECRET") == "" {
		t.Fatal("env vars GODADDY_API_KEY and GODADDY_API_SECRET must be set")
	}
}

// provider instantiation for acceptance tests: use real API
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	// pass "test" as version to the provider constructor
	"godaddy-dns": providerserver.NewProtocol6WithError(New(
		"test",
		func(apiURL, apiKey, apiSecret string) (model.DNSApiClient, error) {
			return client.NewClient(apiURL, apiKey, apiSecret)
		})()),
}

// provider instantiation for unit tests: use mock API
func mockClientProviderFactory(c *model.MockDNSApiClient) map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		// pass "unittest" as version to the provider constructor
		"godaddy-dns": providerserver.NewProtocol6WithError(New(
			"unittest",
			func(apiURL, apiKey, apiSecret string) (model.DNSApiClient, error) {
				return model.DNSApiClient(c), nil
			})()),
	}
}

// make record name, test record and terraform resource name for record type
func makeMockRec(mType model.DNSRecordType, mData model.DNSRecordData) (model.DNSRecordType, model.DNSRecordName, []model.DNSRecord, string) {
	mName := model.DNSRecordName("test-" + strings.ToLower(string(mType)) + "._test")
	mRec := []model.DNSRecord{{
		Name: mName,
		Type: mType,
		Data: mData,
		TTL:  3600,
	}}
	if mType == model.REC_MX {
		mRec[0].Priority = 10
	}
	tfResName := "godaddy-dns_record.test-" + strings.ToLower(string(mType))
	return mType, mName, mRec, tfResName
}

// create standard terraform config for test record of given type
func simpleResourceConfig(rectype model.DNSRecordType, target model.DNSRecordData) string {
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
		string(rectype),
		string(target),
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

// - terraform config witn N records
// - domain record name for it ("test-<type>._test")
// - terraform resource name for record type ("godaddy-dns_record.test-<type>")
// - []model.DNSRecord array with all attrs
// - []model.DNSUpdateRecord w/o name and type
type testRecSet struct {
	TFConfig   string
	DNSRecName model.DNSRecordName
	TFResName  string
	Records    []model.DNSRecord
	UpdRecords []model.DNSUpdateRecord
}

// create terraform config for N record with the same name but different values
func makeTestRecSet(rectype model.DNSRecordType, values []model.DNSRecordData) testRecSet {
	res := testRecSet{}

	templateString := `
	provider "godaddy-dns" {}
	locals {
	  dataValues = [{{ .RecValsJoined }}]
	}
	resource "godaddy-dns_record" "{{ .RecName }}" {
	  domain = "{{ .Domain }}"
	  type   = "{{ .RecType | upper }}"
	  name   = "{{ .DNSRecName }}"

	  count  = length(local.dataValues)
	  data   = local.dataValues[count.index]

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
		return res
	}

	recName := "test-" + strings.ToLower(string(rectype))
	var recPrio model.DNSRecordPrio
	if rectype == model.REC_MX {
		recPrio = 10
	}
	var buff strings.Builder
	valStrings := lo.Map(values, func(x model.DNSRecordData, _ int) string {
		return fmt.Sprintf("\"%v\"", x)
	})
	valStringsJoined := strings.Join(valStrings, ", ")
	resConf := struct {
		Domain        string
		RecType       string
		RecValsJoined *string
		RecName       string
		DNSRecName    string
		Priority      int
	}{
		TEST_DOMAIN,
		string(rectype),
		&valStringsJoined,
		recName,
		recName + "._test",
		-1,
	}
	if rectype == model.REC_MX {
		resConf.Priority = 10
	}
	err = tmpl.ExecuteTemplate(&buff, "config", resConf)
	if err != nil {
		return res
	}
	res.TFConfig = buff.String()
	res.DNSRecName = model.DNSRecordName(recName + "._test")
	res.TFResName = "godaddy-dns_record." + recName
	res.Records = lo.Map(values, func(data model.DNSRecordData, _ int) model.DNSRecord {
		return model.DNSRecord{
			Name:     res.DNSRecName,
			Type:     rectype,
			Data:     data,
			TTL:      3600,
			Priority: recPrio,
		}
	})
	res.UpdRecords = lo.Map(values, func(data model.DNSRecordData, _ int) model.DNSUpdateRecord {
		return model.DNSUpdateRecord{
			Data:     data,
			TTL:      3600,
			Priority: recPrio,
		}
	})
	return res
}

// check that actual record (from API query) matches resource state
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
