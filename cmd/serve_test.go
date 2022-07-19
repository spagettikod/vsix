package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spagettikod/vsix/database"
)

func TestCORSQuery(t *testing.T) {
	expectedHeaders := "test,hepp"

	memdb, err := database.OpenMem()
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodOptions, "https://www.foo.bar/testing", nil)
	req.Header.Add("Access-Control-Request-Headers", expectedHeaders)
	rec := httptest.NewRecorder()
	handler := queryHandler(memdb, "https://www.foo.bar", "/testing")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %v but got %v", http.StatusOK, rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Headers") != expectedHeaders {
		t.Fatalf("expected header 'Access-Control-Allow-Headers: %s' but got 'Access-Control-Allow-Headers: %s'", expectedHeaders, rec.Header().Get("Access-Control-Allow-Headers"))
	}
}

func Test_URLParse(t *testing.T) {
	type Test struct {
		ExternalURL          string
		ExpectedServer       string
		ExpectedAPIRoot      string
		ExpectedAssetRoot    string
		ExternalURLIsInvalid bool
	}

	tests := []Test{
		{
			ExternalURL:          "https://www.example.com:8080",
			ExpectedServer:       "https://www.example.com:8080",
			ExpectedAPIRoot:      "/extensionquery",
			ExpectedAssetRoot:    "/" + assetURLPath,
			ExternalURLIsInvalid: false,
		},
		{
			ExternalURL:          "https://www.example.com/hepp",
			ExpectedServer:       "https://www.example.com",
			ExpectedAPIRoot:      "/hepp/extensionquery",
			ExpectedAssetRoot:    "/hepp/" + assetURLPath,
			ExternalURLIsInvalid: false,
		},
		{
			ExternalURL:          "www.example.com/hepp",
			ExternalURLIsInvalid: true,
		},
		{
			ExternalURL:          "http://www.example.com/hepp",
			ExpectedServer:       "http://www.example.com",
			ExpectedAPIRoot:      "/hepp/extensionquery",
			ExpectedAssetRoot:    "/hepp/" + assetURLPath,
			ExternalURLIsInvalid: false,
		},
		{
			ExternalURL:          "http://localhost:8080",
			ExpectedServer:       "http://localhost:8080",
			ExpectedAPIRoot:      "/extensionquery",
			ExpectedAssetRoot:    "/" + assetURLPath,
			ExternalURLIsInvalid: false,
		},
	}

	for _, test := range tests {
		s, u, a, err := parseEndpoints(test.ExternalURL)
		if err != nil {
			if !test.ExternalURLIsInvalid {
				t.Errorf("test %s: URL %s was invalid but was supposed to be valid, got error: %v", test.ExternalURL, test.ExternalURL, err)
			}
			continue
		}
		if s != test.ExpectedServer {
			t.Errorf("test %s: Server was %s, expected %s", test.ExternalURL, s, test.ExpectedServer)
			continue
		}
		if u != test.ExpectedAPIRoot {
			t.Errorf("test %s: APIRoot was %s, expected %s", test.ExternalURL, u, test.ExpectedAPIRoot)
			continue
		}
		if a != test.ExpectedAssetRoot {
			t.Errorf("test %s: AssetRoot was %s, expected %s", test.ExternalURL, a, test.ExpectedAssetRoot)
			continue
		}
	}
}
