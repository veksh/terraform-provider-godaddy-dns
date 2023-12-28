//go:generate mockery --all

package model

import "context"

type DNSDomain string

type DNSRecordType string
type DNSRecordName string
type DNSRecordData string
type DNSRecordTTL uint32 // formally int32, but [0, 604800]
type DNSRecordPrio uint16
type DNSRecordSRVWeight uint16
type DNSRecordSRVProto string   // _tcp or _udp
type DNSRecordSRVService string // _ldap
type DNSRecordSRVPort uint16

const (
	REC_A     = DNSRecordType("A")
	REC_AAAA  = DNSRecordType("AAAA")
	REC_CNAME = DNSRecordType("CNAME")
	REC_MX    = DNSRecordType("MX")
	REC_NS    = DNSRecordType("NS")
	REC_SOA   = DNSRecordType("SOA")
	REC_SRV   = DNSRecordType("SRV")
	REC_TXT   = DNSRecordType("TXT")
)

type DNSRecord struct {
	Name     DNSRecordName // @ for top-level TXT/MX/A/NS
	Type     DNSRecordType // from the enum above
	Data     DNSRecordData // "Parked" for top-level "A" (name "@")
	TTL      DNSRecordTTL  // min 600, def 3600
	Priority DNSRecordPrio // MX and SRV

	Weight   DNSRecordSRVWeight  // SRV
	Protocol DNSRecordSRVProto   // SRV: _tcp or _udp
	Service  DNSRecordSRVService // SRV: like _ldap
	Port     DNSRecordSRVPort    // SRV, 1-65535
}

// compare key field to determine if two records refer to the same object
// - for A and CNAME there could be only 1 RR with the same name, TTL is the only value
// - for TXT and NS there could be several (so need to match by data),
// - MX matches the same way, value is ttl + prio
// - and SRV same if Protocol, Port, Service and Data are matched
func (r DNSRecord) SameKey(r1 DNSRecord) bool {
	if r.Type != r1.Type || r.Name != r1.Name {
		return false
	}
	if r.Type == REC_CNAME || r.Type == REC_A || r.Type == REC_AAAA {
		return true
	}
	if r.Type == REC_TXT || r.Type == REC_MX || r.Type == REC_NS {
		return r.Data == r1.Data
	}
	if r.Type == REC_SRV {
		return r.Protocol == r1.Protocol && r.Service == r1.Service && r.Data == r1.Data
	}
	// soa?
	return false
}

// client
type DNSApiClient interface {
	AddRecords(ctx context.Context, domain DNSDomain, records []DNSRecord) error
	GetRecords(ctx context.Context, domain DNSDomain, rType DNSRecordType, rName DNSRecordName) ([]DNSRecord, error)
	SetRecords(ctx context.Context, domain DNSDomain, rType DNSRecordType, rName DNSRecordName, records []DNSRecord) error
	DelRecords(ctx context.Context, domain DNSDomain, rType DNSRecordType, rName DNSRecordName) error
}
