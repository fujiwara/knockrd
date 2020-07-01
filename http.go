package knockrd

import (
	crand "crypto/rand"
	"fmt"
	"html/template"
	"log"
	"net/http"

	_ "github.com/fujiwara/knockrd/statik"
	"github.com/rakyll/statik/fs"
)

type View struct {
	IPAddr    string
	CSRFToken string
	Message   string
}

var (
	mux     = http.NewServeMux()
	backend Backend
	tmpl    = template.Must(template.New("view").Parse(`<!DOCTYPE html>
<html>
  <head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>knockrd</title>
	<link rel="stylesheet" href="/public/css/pure-min.css">
  </head>
  <body style="padding: 1em;">
    <div class="pure-g">
      <div class="pure-u">
		<h1>knockrd</h1>
		<p>Your IP address <strong>{{ .IPAddr }}</strong> {{ .Message }}</p>
		{{ if ne .CSRFToken "" }}
        <form class="pure-form pure-form-stacked" method="POST">
          <fieldset>
            <input type="hidden" name="csrf_token" value="{{ .CSRFToken }}">
            <button type="submit" name="allow" value="allow" class="pure-button pure-button-primary">Allow</button>
            <button type="submit" name="disallow" value="disallow" class="pure-button">Disallow</button>
          </fieldset>
		</form>
		{{ end }}
      </div>
    </div>
  </body>
</html>
`))
)

var httpHandlerFuncs = make(map[string]handlerFunc)

func init() {
	statikFS, err := fs.New()
	if err != nil {
		panic(err)
	}
	httpHandlerFuncs["/"] = rootHandler
	httpHandlerFuncs["/allow"] = allowHandler
	httpHandlerFuncs["/auth"] = authHandler
	mux.Handle("/public/", http.StripPrefix("/public/", http.FileServer(statikFS)))
}

type handlerFunc func(http.ResponseWriter, *http.Request) error

type allowFunc func(r *http.Request) (bool, error)

func wrapHandlerFunc(h handlerFunc, allow allowFunc) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "private")

		if allow != nil {
			if ok, err := allow(r); err != nil {
				log.Println("[error]", err)
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintln(w, "Server Error")
				return
			} else if !ok {
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprintln(w, "Forbidden")
				return
			}
		}

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
	token, err := csrfToken()
	if err != nil {
		return err
	}
	if err := backend.Set(token); err != nil {
		return err
	}
	return render(w, View{
		IPAddr:    ipaddr,
		CSRFToken: token,
	})
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

	var message string
	if r.FormValue("allow") != "" {
		log.Println("[debug] setting allowed IP address", ipaddr)
		if err := backend.Set(ipaddr); err != nil {
			return err
		}
		log.Printf("[info] set allowed IP address for %s TTL %s", ipaddr, backend.TTL())
		message = fmt.Sprintf("is allowed for %s.", backend.TTL())
	} else if r.FormValue("disallow") != "" {
		log.Println("[debug] removing allowed IP address", ipaddr)
		if err := backend.Delete(ipaddr); err != nil {
			return err
		}
		log.Println("[info] remove allowed IP address", ipaddr)
		message = "is disallowed."
	} else {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Bad request")
		return nil
	}
	return render(w, View{
		IPAddr:  ipaddr,
		Message: message,
	})
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
	return render(w, View{IPAddr: r.Header.Get("X-Real-IP")})
}

func csrfToken() (string, error) {
	k := make([]byte, 32)
	if _, err := crand.Read(k); err != nil {
		return "", err
	}
	return noCachePrefix + fmt.Sprintf("%x", k), nil
}

func render(w http.ResponseWriter, v View) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return tmpl.ExecuteTemplate(w, "view", v)
}
