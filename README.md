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
ttl: 300             # Expiration(sec) for allowed access
real_ip_from:
  - 10.0.0.0/8
```

## Usage with Nginx auth_request directive

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

### Authorization Flow

1. A user accesses to `/allow`.
   - This location must be protected by other methods like OAuth or etc.
1. knockrd shows HTML form to allow access from the user's IP address.
1. The user pushes the "Allow" button.
1. knockrd store the IP address to the backend(DynamoDB).
1. The user accesses to other locations.
1. Nginx auth_request directive requests to knockrd `/auth`.
1. knockrd compares the IP address with the backend and returns 200 OK or 403 Forbidden.
1. Nginx allows or denies the user's request based on the knockrd response.

## Usage with AWS WAF v2 IP Set (serverless)

Prepare an IAM Role for lambda functions. `arn:aws:iam::{your account ID}:role/knockrd_lambda`
The role must have policies which allows actions as below.

- wafv2:GetIPSet
- wafv2:UpdateIPSet
- dynamodb:GetItem
- dynamodb:PutItem
- dynamodb:UpdateItem
- dynamodb:CreateTable
- dynamodb:UpdateTimeToLive

Prepare IP sets for AWF WAF v2.

Prepare config.yaml for the IP sets.

```yaml
ttl: 300s
ip-set:
  v4:
    id: ddcdf8ad-251c-4c8f-b12f-05628b87beb6
    name: knockrd
    scope: REGIONAL
```

Deploy two lambda functions, knockrd-http and knockrd-stream in [lambda directory](https://github.com/fujiwara/knockrd/tree/master/lambda) with the IAM role and config.yaml. The example of lambda directory uses [lambroll](https://github.com/fujiwara/lambroll) for deployment.

### Authorization Flow

1. A user accesses to `/allow` provided by knockrd-http.
   - This location must be protected by other methods like OAuth or etc.
1. knockrd-http shows HTML form to allow access from the user's IP address.
1. The user pushes the "Allow" button.
1. knockrd-http store the IP address to the backend(DynamoDB).
    - knockrd-stream updates IP set by events on a dynamodb stream.
1. The user accesses to other locations.
1. AWS WAF allows or denies the user's request based on the ip sets.

## LICENSE

MIT License

Copyright (c) 2020 FUJIWARA Shunichiro
