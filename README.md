# knockrd

HTTP knocker daemon.

## Usage

```console
Usage of knockrd:
  -config string
    	config file name
  -debug
    	enable debug log
```

```yaml
port: 9876
table_name: knockrd  # DynamoDB table name
expires: 300         # Expiration(sec) for allowed access
real_ip_from:
  - 10.0.0.0/8
```

### With Nginx auth_request directive

Nginx example configuration.

```
http {
    server {
       listen 80;
       location = /allow {
           proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
           proxy_pass http://127.0.0.1:9876;
       }
       location = /auth {
           internal;
           proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
           proxy_pass http://127.0.0.1:9876;
       }
       location / {
           auth_request /auth;
           root /var/www/htdocs;
       }
    }
}
```

#### Authorization Flow

1. A user accesses to `/allow`.
1. knockrd shows HTML form to allow access from the user's IP address.
1. The user pushes the "Allow" button.
1. knockrd store the IP address to the backend(DynamoDB).
1. The user accesses to other locations.
1. Nginx auth_request directive requests to knockrd `/auth`.
1. knockrd compares the IP address with the backend and returns 200 OK or 403 Forbidden.
1. Nginx allows or denies the user's request by the knockrd response.

## LICENSE

MIT License

Copyright (c) 2020 FUJIWARA Shunichiro
