package knockrd_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fujiwara/knockrd"
)

func TestGetRealIPAddr(t *testing.T) {
	testCases := []struct {
		desc        string
		headerValue string
		expectedIP  string
		expectError bool
	}{
		{
			desc:        "valid IPv4 address with port number",
			headerValue: "192.0.2.1:12345",
			expectedIP:  "192.0.2.1",
			expectError: false,
		},
		{
			desc:        "valid IPv4 address without port number",
			headerValue: "192.0.2.1",
			expectedIP:  "192.0.2.1",
			expectError: false,
		},
		{
			desc:        "valid IPv6 address with port number",
			headerValue: "[2001:db8::1]:12345",
			expectedIP:  "2001:db8::1",
			expectError: false,
		},
		{
			desc:        "valid IPv6 address without port number",
			headerValue: "2001:db8::1",
			expectedIP:  "2001:db8::1",
			expectError: false,
		},
		{
			desc:        "invalid IP address",
			headerValue: "invalid-ip-address",
			expectedIP:  "",
			expectError: true,
		},
		{
			desc:        "empty header value",
			headerValue: "",
			expectedIP:  "",
			expectError: true,
		},
	}

	for _, tC := range testCases {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Real-IP", tC.headerValue)

		ipAddr, err := knockrd.GetRealIPAddr(req)

		if tC.expectError && err == nil {
			t.Errorf("%s: expected an error, but got none", tC.desc)
		} else if !tC.expectError && err != nil {
			t.Errorf("%s: unexpected error: %s", tC.desc, err)
		} else if !tC.expectError && ipAddr != tC.expectedIP {
			t.Errorf("%s: expected IP address %s, but got %s", tC.desc, tC.expectedIP, ipAddr)
		}
	}
}
