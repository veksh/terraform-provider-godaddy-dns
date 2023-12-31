package provider

import (
	"context"
	"fmt"
	"html/template"
	"strings"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/pkg/errors"
	"github.com/veksh/terraform-provider-godaddy-dns/internal/client"
	"github.com/veksh/terraform-provider-godaddy-dns/internal/model"
)

const TEST_DOMAIN = "veksh.in"

// make standard dns record name and terraform resource name out of record type
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
