package dcstodcs

import (
	"net/http"
	"os"
	"strings"

	dcstodcs "digital-contracting-service/gen/dcs_to_dcs"
	dcstodcsc "digital-contracting-service/gen/http/dcs_to_dcs/client"
	"digital-contracting-service/internal/base/conf"

	goahttp "goa.design/goa/v3/http"
)

type prefixDoer struct {
	prefix string
	inner  goahttp.Doer
}

func (d *prefixDoer) Do(req *http.Request) (*http.Response, error) {
	req.URL.Path = d.prefix + req.URL.Path
	return d.inner.Do(req)
}

// NewDCSToDCSHttpClient builds the Goa-generated DCS-to-DCS client used to call
// post_pdf/get_provenance on a remote peer resolved from its did:web identifier
// (see identity.DIDWebPath). Requests are authenticated via a per-call did:web
// challenge-response signature carried in the request body, not via this HTTP
// client — there is no bearer token here.
//
// pathPrefix carries the peer's own did:web path segments, so a peer sharing a
// host with other instances is still addressed at its own base rather than
// whichever instance happens to own the host root.
func NewDCSToDCSHttpClient(host string, pathPrefix string) *dcstodcs.Client {
	apiPath := os.Getenv("DCS_API_PATH")
	if apiPath == "" {
		apiPath = "/"
	}
	apiPath = strings.TrimSuffix(pathPrefix, "/") + apiPath
	httpClient := &http.Client{Timeout: conf.HTTPClientTimeout()}
	doer := &prefixDoer{prefix: apiPath, inner: httpClient}

	c := dcstodcsc.NewClient(
		"http",
		host,
		doer,
		goahttp.RequestEncoder,
		goahttp.ResponseDecoder,
		false,
	)
	postPdf := c.PostPdf()
	getProvenance := c.GetProvenance()
	return dcstodcs.NewClient(postPdf, getProvenance)
}
