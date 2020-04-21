package knockrd

import (
	crand "crypto/rand"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/fujiwara/ridge"
	"github.com/natureglobal/realip"
)

var (
	mux     = http.NewServeMux()
	backend Backend
	tmpl    = template.Must(template.New("form").Parse(`<!DOCTYPE html>
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
	  <input type="submit" value="Allow" name="allow">
	  <input type="submit" value="Disallow" name="disallow">
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

type lambdaHandler struct {
	handler http.Handler
}

func (h lambdaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("[debug] remote_addr:%s", r.RemoteAddr)
	log.Printf("[debug] headers:%#v", r.Header)
	r.RemoteAddr = "127.0.0.1:0"
	h.handler.ServeHTTP(w, r)
}

// Run runs knockrd
func Run(conf *Config) error {
	handler, err := configure(conf)
	if err != nil {
		return err
	}
	addr := fmt.Sprintf(":%d", conf.Port)
	log.Printf("[info] knockrd starting up on %s", addr)
	ridge.Run(addr, "/", handler)
	return nil
}

func configure(conf *Config) (http.Handler, error) {
	log.Println("[debug] configure")
	var ipfroms []*net.IPNet
	for _, cidr := range conf.RealIPFrom {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, err
		}
		ipfroms = append(ipfroms, ipnet)
	}
	middleware := realip.MustMiddleware(&realip.Config{
		RealIPFrom:      ipfroms,
		RealIPHeader:    conf.RealIPHeader,
		RealIPRecursive: true,
	})
	handler := middleware(mux)
	if env := os.Getenv("AWS_EXECUTION_ENV"); strings.HasPrefix(env, "AWS_Lambda_go") {
		log.Printf("[debug] configure for %s", env)
		handler = lambdaHandler{handler}
	}

	var err error
	if b, err := NewDynamoDBBackend(conf); err != nil {
		return nil, err
	} else {
		backend, err = NewCachedBackend(b, conf.TTL, conf.NegativeTTL)
	}
	return handler, err
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
	w.Header().Set("Cache-Control", "private")
	ipaddr := r.Header.Get("X-Real-IP")
	if ipaddr == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Bad request")
		return nil
	}
	token, err := csrfToken()
	if err != nil {
		return err
	}
	if err := backend.Set(token); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = tmpl.ExecuteTemplate(w, "form",
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
	w.Header().Set("Cache-Control", "private")
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

	if r.FormValue("allow") != "" {
		log.Println("[debug] setting allowed IP address", ipaddr)
		if err := backend.Set(ipaddr); err != nil {
			return err
		}
		log.Println("[info] set allowed IP address", ipaddr)
		fmt.Fprintf(w, "Allowed from %s\n", ipaddr)
	} else if r.FormValue("disallow") != "" {
		log.Println("[debug] removing allowed IP address", ipaddr)
		if err := backend.Delete(ipaddr); err != nil {
			return err
		}
		log.Println("[info] remove allowed IP address", ipaddr)
		fmt.Fprintf(w, "Disallowed from %s\n", ipaddr)
	}
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

func csrfToken() (string, error) {
	k := make([]byte, 32)
	if _, err := crand.Read(k); err != nil {
		return "", err
	}
	return fmt.Sprintf("__%x", k), nil
}
