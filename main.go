package main

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	tcp "tcping/tcping"
	"tcping/tcping/http"
	"tcping/tcping/ping"
)

var (
	counter  int    = 1
	timeout  string = "1s"
	interval string = "0s"
	sigs     chan os.Signal

	httpMethod string = "GET"
)

func fixProxy(proxy string, op *ping.Option) error {
	if proxy == "" {
		return nil
	}
	u, err := url.Parse(proxy)
	op.Proxy = u
	return err
}

func init() {
	ua := "tcping"
	meta := false
	proxy := ""
	ping.Register(ping.HTTP, func(url *url.URL, op *ping.Option) (ping.Ping, error) {
		if err := fixProxy(proxy, op); err != nil {
			return nil, err
		}
		op.UA = ua
		return http.New(httpMethod, url.String(), op, meta)
	})
	ping.Register(ping.HTTPS, func(url *url.URL, op *ping.Option) (ping.Ping, error) {
		if err := fixProxy(proxy, op); err != nil {
			return nil, err
		}
		op.UA = ua
		return http.New(httpMethod, url.String(), op, meta)
	})
	ping.Register(ping.TCP, func(url *url.URL, op *ping.Option) (ping.Ping, error) {
		port, err := strconv.Atoi(url.Port())
		if err != nil {
			return nil, err
		}
		return tcp.New(url.Hostname(), port, op, meta), nil
	})

}

func tc(arg string) (string, error) {
	url, _ := ping.ParseAddress(arg)

	defaultPort := "80"
	if port := url.Port(); port != "" {
		defaultPort = port
	} else if url.Scheme == "https" {
		defaultPort = "443"
	}
	port, _ := strconv.Atoi(defaultPort)
	url.Host = fmt.Sprintf("%s:%d", url.Hostname(), port)

	timeoutDuration, _ := ping.ParseDuration(timeout)

	intervalDuration, _ := ping.ParseDuration(interval)

	protocol, _ := ping.NewProtocol(url.Scheme)

	option := ping.Option{
		Timeout: timeoutDuration,
	}
	pingFactory := ping.Load(protocol)
	p, _ := pingFactory(url, &option)

	pinger := ping.NewPinger(os.Stdout, url, p, intervalDuration, counter)
	sigs = make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go pinger.Ping()
	select {
	case <-sigs:
	case <-pinger.Done():
	}
	pinger.Stop()
	return pinger.Summarize()
}

type GfwTest struct{}

func (g *GfwTest) Tcping(url string, reply *string) error {
	msg, e := tc(url)
	*reply = msg
	return e
}

func main() {
	rpc.RegisterName("GfwTest", new(GfwTest))
	listen, _ := net.Listen("tcp", ":1234")
	for {
		conn, err := listen.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go rpc.ServeConn(conn)
	}
}
