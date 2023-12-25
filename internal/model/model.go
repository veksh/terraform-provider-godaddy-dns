package model

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
