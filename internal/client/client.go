package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"github.com/veksh/terraform-provider-godaddy-dns/internal/model"
	"github.com/veksh/terraform-provider-godaddy-dns/libs/ratelimiter"
)

// see also: https://github.com/go-resty/resty

// to view actual records
// curlie -v GET "https://api.godaddy.com/v1/domains/<domain>/records" -H "Authorization: sso-key $GODADDY_API_KEY:$GODADDY_API_SECRET"

var _ DNSApiClient = Client{}

type DNSApiClient interface {
	AddRecords(ctx context.Context, domain model.DNSDomain, records []model.DNSRecord) error
	GetRecords(ctx context.Context, domain model.DNSDomain, rType model.DNSRecordType, rName model.DNSRecordName) ([]model.DNSRecord, error)
	SetRecords(ctx context.Context, domain model.DNSDomain, rType model.DNSRecordType, rName model.DNSRecordName, records []model.DNSRecord) error
	DelRecords(ctx context.Context, domain model.DNSDomain, rType model.DNSRecordType, rName model.DNSRecordName) error
}

const (
	HTTP_TIMEOUT = 10
	HTTP_RPS     = 1
	HTTP_BURST   = 60
	DOMAINS_URL  = "/v1/domains/"
)

type rateLimitedHTTPTransport struct {
	limiter *ratelimiter.RateLimiter
	next    http.RoundTripper
}

func (t *rateLimitedHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.limiter.Wait()
	return t.next.RoundTrip(req)
}

// mb also http client here
type Client struct {
	apiURL     string
	key        string
	secret     string
	httpClient http.Client
}

func NewClient(apiURL string, key string, secret string) (*Client, error) {
	// t := http.DefaultTransport.(*http.Transport).Clone()
	httpTransport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: HTTP_TIMEOUT * time.Second}).DialContext,
		TLSHandshakeTimeout:   HTTP_TIMEOUT * time.Second,
		ResponseHeaderTimeout: HTTP_TIMEOUT * time.Second,
		MaxIdleConns:          10,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       10,
		IdleConnTimeout:       60,
	}
	rateLimiter, err := ratelimiter.New(HTTP_RPS, HTTP_BURST)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create rate limiter")
	}
	httpClient := http.Client{
		Timeout: HTTP_TIMEOUT * time.Second,
		Transport: &rateLimitedHTTPTransport{
			limiter: rateLimiter,
			next:    httpTransport,
		},
	}
	return &Client{
		apiURL:     apiURL,
		key:        key,
		secret:     secret,
		httpClient: httpClient,
	}, nil
}

// see API docs: https://developer.godaddy.com/doc/endpoint/domains/
// ok for both Get and Add (patch), Put (replace) is partial
// first 4 are always present, rest are only for MX (priority) and SRV
type apiDNSRecord struct {
	Data     string `json:"data,omitempty"`
	Name     string `json:"name"`
	Type     string `json:"type,omitempty"`
	TTL      uint32 `json:"ttl,omitempty"`
	Priority uint16 `json:"priority,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	Service  string `json:"service,omitempty"`
	Port     uint16 `json:"port,omitempty"`
	Weight   uint16 `json:"weight,omitempty"`
}

type apiErrorResponce struct {
	Error   string `json:"code"`    // like "INVALID_VALUE_ENUM"
	Message string `json:"message"` // like "type not any of: A, ..."
}

func (c Client) makeRecordsRequest(ctx context.Context, path string, method string, body io.Reader) (*http.Response, error) {

	requestURL, _ := url.JoinPath(c.apiURL, DOMAINS_URL, path)

	ctx, fnCancel := context.WithTimeout(ctx, HTTP_TIMEOUT*time.Second)
	defer fnCancel()

	req, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create request")
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("sso-key %s:%s", c.key, c.secret))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "http request error")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		var errRes apiErrorResponce
		if err = json.NewDecoder(resp.Body).Decode(&errRes); err == nil {
			return nil, errors.New("api error: " + errRes.Message)
		}
		return nil, fmt.Errorf("bad http reply status (%s)", resp.Status)
	}
	return resp, nil
}

// in real API call
// - name and then type are optional (to get all records of type or just all records)
// - there are also "offset" and "limit" in query params for paged output
func (c Client) GetRecords(ctx context.Context, rDomain model.DNSDomain,
	rType model.DNSRecordType, rName model.DNSRecordName) ([]model.DNSRecord, error) {

	rPath, _ := url.JoinPath(string(rDomain), "records", string(rType), string(rName))

	resp, err := c.makeRecordsRequest(ctx, rPath, http.MethodGet, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var responceRecords []apiDNSRecord
	err = json.NewDecoder(resp.Body).Decode(&responceRecords)
	if err != nil {
		return nil, errors.Wrap(err, "cannot decode json reply")
	}
	res := make([]model.DNSRecord, 0, len(responceRecords))
	for _, rr := range responceRecords {
		res = append(res, model.DNSRecord{
			Name:     model.DNSRecordName(rr.Name),
			Type:     model.DNSRecordType(rr.Type),
			Data:     model.DNSRecordData(rr.Data),
			TTL:      model.DNSRecordTTL(rr.TTL),
			Priority: model.DNSRecordPrio(rr.Priority),
			Weight:   model.DNSRecordSRVWeight(rr.Weight),
			Protocol: model.DNSRecordSRVProto(rr.Protocol),
			Service:  model.DNSRecordSRVService(rr.Service),
			Port:     model.DNSRecordSRVPort(rr.Port),
		})
	}
	return res, nil
}

// create (add) records for rType+rName
// existing are staying in place; there could be several records for type + name (eg MX)
func (c Client) AddRecords(ctx context.Context, rDomain model.DNSDomain,
	records []model.DNSRecord) error {

	rPath, _ := url.JoinPath(string(rDomain), "records")

	recs := make([]apiDNSRecord, 0, len(records))
	for _, mr := range records {
		rec := apiDNSRecord{
			Data: string(mr.Data),
			Name: string(mr.Name),
			Type: string(mr.Type),
			TTL:  uint32(mr.TTL),
		}
		if mr.Type == model.REC_MX || mr.Type == model.REC_SRV {
			rec.Priority = uint16(mr.Priority)
		}
		if mr.Type == model.REC_SRV {
			rec.Protocol = string(mr.Protocol)
			rec.Service = string(mr.Service)
			rec.Port = uint16(mr.Port)
			rec.Weight = uint16(mr.Weight)
		}
		recs = append(recs, rec)
	}

	jsonData, err := json.Marshal(&recs)
	if err != nil {
		return errors.Wrap(err, "cannot marshal json")
	}

	resp, err := c.makeRecordsRequest(ctx, rPath, http.MethodPatch, bytes.NewReader(jsonData))
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return err
	}

	return nil
}

// replace all records for rType+rName with the given
//   - data and name fields in dns records must be blank
//   - there could be several records with the same type + name (eg MX)
//     and there is no way to update just one: they all get replaced
func (c Client) SetRecords(ctx context.Context, rDomain model.DNSDomain,
	rType model.DNSRecordType, rName model.DNSRecordName, records []model.DNSRecord) error {

	// API will reject them anyway
	for _, r := range records {
		if r.Type != "" || r.Name != "" {
			return fmt.Errorf("data and name in records should be blank")
		}
	}

	rPath, _ := url.JoinPath(string(rDomain), "records", string(rType), string(rName))

	recs := make([]apiDNSRecord, 0, len(records))
	for _, mr := range records {
		rec := apiDNSRecord{
			Data: string(mr.Data),
			TTL:  uint32(mr.TTL),
		}
		if mr.Type == model.REC_MX || mr.Type == model.REC_SRV {
			rec.Priority = uint16(mr.Priority)
		}
		if mr.Type == model.REC_SRV {
			rec.Protocol = string(mr.Protocol)
			rec.Service = string(mr.Service)
			rec.Port = uint16(mr.Port)
			rec.Weight = uint16(mr.Weight)
		}
		recs = append(recs, rec)
	}

	jsonData, err := json.Marshal(&recs)
	if err != nil {
		return errors.Wrap(err, "cannot marshal json")
	}

	resp, err := c.makeRecordsRequest(ctx, rPath, http.MethodPut, bytes.NewReader(jsonData))
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return err
	}

	return nil
}

// delete all records for this type + name (no way to delete e.g. only 1 MX)
func (c Client) DelRecords(ctx context.Context, rDomain model.DNSDomain, rType model.DNSRecordType, rName model.DNSRecordName) error {

	rPath, _ := url.JoinPath(string(rDomain), "records", string(rType), string(rName))

	resp, err := c.makeRecordsRequest(ctx, rPath, http.MethodDelete, nil)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return err
	}

	return nil
}
