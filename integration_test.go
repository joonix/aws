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
	instance    = flag.String("instance", "i-7ae3b239", "Instance id to run experiments on")
	volume      = flag.String("volume", "vol-9d351996", "Volume id to run experiments with")
	endpoint    = flag.String("endpoint", "https://ec2.eu-west-1.amazonaws.com", "AWS Endpoint to use")
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
	ebs, err := NewEbsClient(client, *endpoint, defaultSigner)
	if err != nil {
		t.Error(err)
	}
	vols, err := ebs.VolumesByTags([]TagItem{TagItem{"Name", "test"}})
	if err != nil {
		t.Error(err)
	}
	t.Log(vols)
}

func TestVolumeByIdIntegration(t *testing.T) {
	if !*integration {
		t.Skip("Integration tests not enabled")
		return
	}

	client := &http.Client{Transport: newLoggingTransport()}
	ebs, err := NewEbsClient(client, *endpoint, defaultSigner)
	if err != nil {
		t.Error(err)
	}
	vol, err := ebs.VolumeById(*volume)
	if err != nil {
		t.Error(err)
	}
	if vol.Status != VolumeAvailable {
		t.Error("Expected volume status to be", VolumeAvailable)
	}

	t.Log(vol)
}

func TestCreateVolumeIntegration(t *testing.T) {
	if !*integration {
		t.Skip("Integration tests not enabled")
		return
	}

	client := &http.Client{Transport: newLoggingTransport()}
	ebs, err := NewEbsClient(client, *endpoint, defaultSigner)
	if err != nil {
		t.Error(err)
	}

	tags := []TagItem{
		TagItem{"Name", "test"},
		TagItem{"Stack", "joonix-testing"},
	}
	testVolume, err := ebs.CreateVolume(1, 0, false, "eu-west-1a", "", tags)
	if err != nil {
		t.Error(err)
	}

	t.Log(testVolume)
}

func TestAttachVolumeIntegration(t *testing.T) {
	if !*integration {
		t.Skip("Integration tests not enabled")
		return
	}

	client := &http.Client{Transport: newLoggingTransport()}
	ebs, err := NewEbsClient(client, *endpoint, defaultSigner)
	if err != nil {
		t.Error(err)
	}

	path, err := ebs.AttachVolume(*volume, *instance)
	if err != nil {
		t.Error(err)
	}

	if path != "/dev/sdf" {
		t.Error("Expected path to be set correctly")
	}
}

func TestDetachVolumeIntegration(t *testing.T) {
	if !*integration {
		t.Skip("Integration tests not enabled")
		return
	}

	client := &http.Client{Transport: newLoggingTransport()}
	ebs, err := NewEbsClient(client, *endpoint, defaultSigner)
	if err != nil {
		t.Error(err)
	}

	status, err := ebs.DetachVolume(*volume)
	if err != nil {
		t.Error(err)
	}
	if status != VolumeDetaching {
		t.Error("Expected volume to start detaching")
	}
}
