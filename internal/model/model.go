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
	Type     DNSRecordType // from the enum above
	Name     DNSRecordName // @ for top-level TXT/MX/A/NS
	Data     DNSRecordData // "Parked" for top-level "A" (name "@")
	TTL      DNSRecordTTL  // min 600, def 3600
	Priority DNSRecordPrio // MX and SRV

	Service  DNSRecordSRVService // SRV: like _ldap
	Protocol DNSRecordSRVProto   // SRV: _tcp or _udp
	Port     DNSRecordSRVPort    // SRV, 1-65535
	Weight   DNSRecordSRVWeight  // SRV
}

type DNSUpdateRecord struct {
	Data     DNSRecordData // "Parked" for top-level "A" (name "@")
	TTL      DNSRecordTTL  // min 600, def 3600
	Priority DNSRecordPrio // MX and SRV

	Service  DNSRecordSRVService // SRV: like _ldap
	Protocol DNSRecordSRVProto   // SRV: _tcp or _udp
	Port     DNSRecordSRVPort    // SRV, 1-65535
	Weight   DNSRecordSRVWeight  // SRV
}

// compare key field to determine if two records refer to the same object
//   - for CNAME there could be only 1 RR with the same name, TTL is the only value
//   - for A, TXT and NS there could be several (so need to match by data),
//   - MX matches the same way, value is ttl + prio (in theory, MX 0 and MX 10
//     could point to the same host in "data", but lets think that it is a perversion
//     and replace it with one record
//   - and SRV same if Protocol, Port, Service and Data are matched
func (r DNSRecord) SameKey(r1 DNSRecord) bool {
	if r.Type != r1.Type || r.Name != r1.Name {
		return false
	}
	if r.Type == REC_CNAME {
		return true
	}
	if r.Type == REC_SRV {
		return r.Protocol == r1.Protocol && r.Service == r1.Service &&
			r.Port == r1.Port && r.Data == r1.Data
	}
	// TXT, MX, NS, A, AAAA
	return r.Data == r1.Data
}

// convert DNSRecord to update format (dropping 2 first fields)
func (r DNSRecord) ToUpdate() DNSUpdateRecord {
	return DNSUpdateRecord{
		Data:     r.Data,
		TTL:      r.TTL,
		Priority: r.Priority,
		Weight:   r.Weight,
		Protocol: r.Protocol,
		Service:  r.Service,
		Port:     r.Port,
	}
}

// true if there is only one possible value for domain+type+key combination
// i.e record is CNAME
func (t DNSRecordType) IsSingleValue() bool {
	return t == REC_CNAME
}

// client API interface
type DNSApiClient interface {
	AddRecords(ctx context.Context, domain DNSDomain, records []DNSRecord) error
	GetRecords(ctx context.Context, domain DNSDomain, rType DNSRecordType, rName DNSRecordName) ([]DNSRecord, error)
	SetRecords(ctx context.Context, domain DNSDomain, rType DNSRecordType, rName DNSRecordName, records []DNSUpdateRecord) error
	DelRecords(ctx context.Context, domain DNSDomain, rType DNSRecordType, rName DNSRecordName) error
}
