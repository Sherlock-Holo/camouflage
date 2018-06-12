package dns

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type Resolver struct {
	Server string

	client dns.Client
}

type Result struct {
	AIP    []net.IP
	AAAAIP []net.IP

	aLock    sync.Mutex
	aaaaLock sync.Mutex
}

var DefaultResolver = Resolver{
	Server: "1.1.1.1:53",
	client: dns.Client{
		Timeout: 30 * time.Second,
	},
}

func NewResolver(server string, net string, timeout time.Duration) Resolver {
	return Resolver{
		Server: server,

		client: dns.Client{
			Net:     net,
			Timeout: timeout,
		},
	}
}

func (r *Resolver) query(host string, ipv6 bool, ctx context.Context) (Result, error) {
	// TODO: edns client subnet

	var msgs []*dns.Msg

	msg := new(dns.Msg)
	msg.Id = dns.Id()
	msg.RecursionDesired = true
	msg.Question = []dns.Question{
		{
			Name:   dns.Fqdn(host),
			Qclass: dns.ClassINET,
			Qtype:  dns.TypeA,
		},
	}

	msgs = append(msgs, msg)

	if ipv6 {
		v6Msg := new(dns.Msg)
		v6Msg.Id = dns.Id()
		v6Msg.RecursionDesired = true
		v6Msg.Question = []dns.Question{
			{
				Name:   dns.Fqdn(host),
				Qclass: dns.ClassINET,
				Qtype:  dns.TypeAAAA,
			},
		}

		msgs = append(msgs, v6Msg)
	}

	var (
		result Result
		group  = sync.WaitGroup{}
	)

	for _, msg := range msgs {
		group.Add(1)
		go func(msg *dns.Msg) {
			defer group.Done()

			rMsg, _, err := r.client.ExchangeContext(ctx, msg, r.Server)
			if err != nil {
				return
			}

			for _, an := range rMsg.Answer {
				switch an := an.(type) {
				case *dns.A:
					result.aLock.Lock()
					result.AIP = append(result.AIP, an.A)
					result.aLock.Unlock()

				case *dns.AAAA:
					result.aaaaLock.Lock()
					result.AAAAIP = append(result.AAAAIP, an.AAAA)
					result.aaaaLock.Unlock()

				case *dns.CNAME:
					cnameResult, err := r.query(an.Target, ipv6, ctx)
					if err == nil {
						result.aLock.Lock()
						result.AIP = append(result.AIP, cnameResult.AIP...)
						result.aLock.Unlock()

						result.aaaaLock.Lock()
						result.AAAAIP = append(result.AAAAIP, cnameResult.AAAAIP...)
						result.aaaaLock.Unlock()
					}
				}
			}
		}(msg)
	}
	group.Wait()

	if len(result.AIP) == 0 && len(result.AAAAIP) == 0 {
		return Result{}, errors.New("resolve failed")
	}

	return result, nil
}

func (r *Resolver) Query(host string, ipv6 bool, timeout time.Duration) (Result, error) {
	var ctx context.Context

	if timeout > 0 {
		ctx, _ = context.WithTimeout(context.Background(), timeout)
	} else {
		ctx = context.Background()
	}

	return r.query(host, ipv6, ctx)
}
