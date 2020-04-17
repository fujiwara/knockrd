package knockrd

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/natureglobal/realip"
)

var (
	mux         = http.NewServeMux()
	middleware  func(http.Handler) http.Handler
	backend     Backend
	defaultCIDR = []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"fe80::/10",
		"::1/128",
	}
)

func init() {
	mux.HandleFunc("/", rootHandler)
	mux.HandleFunc("/allow", allowHandler)
	mux.HandleFunc("/auth", authHandler)
}

// Run runs knockrd
func Run(config *Config) error {
	if err := configure(config); err != nil {
		return err
	}
	addr := fmt.Sprintf(":%d", config.Port)
	log.Printf("[info] knockrd starting up on %s", addr)
	return http.ListenAndServe(addr, middleware(mux))
}

func configure(config *Config) error {
	var ipfroms []*net.IPNet
	var realIPFrom []string
	if len(config.RealIPFrom) == 0 {
		realIPFrom = defaultCIDR
	} else {
		realIPFrom = config.RealIPFrom
	}
	for _, cidr := range realIPFrom {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			return err
		}
		ipfroms = append(ipfroms, ipnet)
	}
	middleware = realip.MustMiddleware(&realip.Config{
		RealIPFrom:      ipfroms,
		RealIPHeader:    realip.HeaderXForwardedFor,
		RealIPRecursive: true,
	})

	var err error
	backend, err = NewDynamoDBBackend(config.TableName, config.AWS.Region, config.AWS.Endpoint)
	return err
}

func allowHandler(w http.ResponseWriter, r *http.Request) {
	ipaddr := r.Header.Get("X-Real-IP")
	if err := backend.Set(ipaddr, time.Now().Unix()+10); err != nil {
		log.Println("[error]", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
	log.Println("[debug] set allowed IP address", ipaddr)
	fmt.Fprintf(w, "%s\n", ipaddr)
}

func authHandler(w http.ResponseWriter, r *http.Request) {
	ipaddr := r.Header.Get("X-Real-IP")
	if ok, err := backend.Get(ipaddr); err != nil {
		log.Println("[error]", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "server error: %s\n", err)
	} else if !ok {
		log.Println("[info] not allowed IP address", ipaddr)
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintln(w, "not allowed")
	} else {
		log.Println("[debug] allowed IP address", ipaddr)
		fmt.Fprintln(w, "ok")
	}
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "knockrd alive from %s\n", r.Header.Get("X-Real-IP"))
}
