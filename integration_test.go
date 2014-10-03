package aws

import (
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"testing"
)

var (
	integration = flag.Bool("integration", false, "Wether to do real AWS requests or not")
)

func init() {
	flag.Parse()
}

type loggingTransport struct {
	*http.Transport
}

func newLoggingTransport() *loggingTransport {
	return &loggingTransport{&http.Transport{}}
}

func (l *loggingTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	b, _ := httputil.DumpRequest(req, true)
	log.Println("\n" + string(b))
	resp, err = l.Transport.RoundTrip(req)
	b, _ = httputil.DumpResponse(resp, true)
	log.Println("\n" + string(b))
	return
}

func TestVolumeByTagsIntegration(t *testing.T) {
	if !*integration {
		t.Skip("Integration tests not enabled")
		return
	}

	client := &http.Client{Transport: newLoggingTransport()}
	ebs, err := NewEbsClient(client, "https://ec2.eu-west-1.amazonaws.com", defaultSigner)
	if err != nil {
		t.Error(err)
	}
	vols, err := ebs.VolumesByTags([]TagItem{TagItem{"Name", "test"}})
	if err != nil {
		t.Error(err)
	}
	t.Log(vols)
}
