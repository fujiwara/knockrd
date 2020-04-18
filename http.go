package knockrd

import (
	crand "crypto/rand"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"

	"github.com/natureglobal/realip"
)

var (
	mux        = http.NewServeMux()
	middleware func(http.Handler) http.Handler
	backend    Backend
	expires    int64
	tmpl       = template.Must(template.New("form").Parse(`<!DOCTYPE html>
<html>
  <head>
    <meta charset="utf-8">
	<title>knockrd</title>
  </head>
  <body>
	<h1>knockrd</h1>
	<p>{{ .IPAddr }}</p>
	<form method="POST">
	  <input type="hidden" name="csrf_token" value="{{ .CSRFToken }}">
	  <input type="submit" value="Allow">
	</form>
  </body>
</html>
`))
)

func init() {
	mux.HandleFunc("/", wrap(rootHandler))
	mux.HandleFunc("/allow", wrap(allowHandler))
	mux.HandleFunc("/auth", wrap(authHandler))
}

type handler func(http.ResponseWriter, *http.Request) error

func wrap(h handler) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		err := h(w, r)
		if err != nil {
			log.Println("[error]", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, "Server Error")
		}
	}
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

func configure(conf *Config) error {
	var ipfroms []*net.IPNet
	for _, cidr := range conf.RealIPFrom {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			return err
		}
		ipfroms = append(ipfroms, ipnet)
	}
	middleware = realip.MustMiddleware(&realip.Config{
		RealIPFrom:      ipfroms,
		RealIPHeader:    conf.RealIPHeader,
		RealIPRecursive: true,
	})

	var err error
	backend, err = NewDynamoDBBackend(conf)
	return err
}

func allowHandler(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case http.MethodGet:
		allowGetHandler(w, r)
	case http.MethodPost:
		allowPostHandler(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
	return nil
}

func allowGetHandler(w http.ResponseWriter, r *http.Request) error {
	ipaddr := r.Header.Get("X-Real-IP")
	if ipaddr == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Bad request")
		return nil
	}
	token := secureRandomString(32)
	if err := backend.Set(token); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := tmpl.ExecuteTemplate(w, "form",
		struct {
			IPAddr    string
			CSRFToken string
		}{
			IPAddr:    ipaddr,
			CSRFToken: token,
		})
	if err != nil {
		return err
	}
	return nil
}

func allowPostHandler(w http.ResponseWriter, r *http.Request) error {
	ipaddr := r.Header.Get("X-Real-IP")
	token := r.FormValue("csrf_token")
	if ipaddr == "" || token == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Bad request")
		return nil
	}

	if ok, err := backend.Get(token); err != nil {
		return err
	} else if !ok {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Bad request")
		return nil
	}
	log.Println("[debug] CSRF token verified")
	if err := backend.Delete(token); err != nil {
		return err
	}

	log.Println("[debug] setting allowed IP address", ipaddr)
	if err := backend.Set(ipaddr); err != nil {
		return err
	}
	log.Println("[info] set allowed IP address", ipaddr)
	fmt.Fprintf(w, "Allowed from %s\n", ipaddr)
	return nil
}

func authHandler(w http.ResponseWriter, r *http.Request) error {
	ipaddr := r.Header.Get("X-Real-IP")
	if ok, err := backend.Get(ipaddr); err != nil {
		return err
	} else if !ok {
		log.Println("[info] not allowed IP address", ipaddr)
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintln(w, "Forbidden")
		return nil
	}
	log.Println("[debug] allowed IP address", ipaddr)
	fmt.Fprintln(w, "OK")
	return nil
}

func rootHandler(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "knockrd alive from %s\n", r.Header.Get("X-Real-IP"))
	return nil
}

func secureRandomString(b int) string {
	k := make([]byte, b)
	if _, err := crand.Read(k); err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x", k)
}
