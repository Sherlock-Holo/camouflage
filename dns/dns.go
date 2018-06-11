package dns

import (
    "errors"
    "net"
    "sync"

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
    Server: "127.0.0.1:53",
    client: dns.Client{
        // Timeout: 2 * time.Second,
    },
}

func (r *Resolver) Query(host string, ipv6 bool) (Result, error) {
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

            rMsg, _, err := r.client.Exchange(msg, r.Server)
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
                    cnameResult, err := r.Query(an.Target, ipv6)
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
