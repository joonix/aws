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
	eip         = flag.String("eip", "54.171.106.174", "Elastic IP for testing")
	instance    = flag.String("instance", "i-7ae3b239", "Instance id to run experiments on")
	volume      = flag.String("volume", "vol-9d351996", "Volume id to run experiments with")
	snapshot    = flag.String("snapshot", "snap-1db38de7", "Snapshot id to run experiments with")
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
	sr := NewSignedRequester(client, *endpoint, DefaultSigner)
	vols, err := VolumesByTags(sr, []TagItem{TagItem{"Name", "test"}})
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
	sr := NewSignedRequester(client, *endpoint, DefaultSigner)
	vol, err := VolumeById(sr, *volume)
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
	sr := NewSignedRequester(client, *endpoint, DefaultSigner)

	tags := []TagItem{
		TagItem{"Name", "test"},
		TagItem{"Stack", "joonix-testing"},
	}
	testVolume, err := CreateVolume(sr, 1, 0, false, "eu-west-1a", "", tags)
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
	sr := NewSignedRequester(client, *endpoint, DefaultSigner)

	path, err := AttachVolume(sr, *volume, *instance)
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
	sr := NewSignedRequester(client, *endpoint, DefaultSigner)

	status, err := DetachVolume(sr, *volume)
	if err != nil {
		t.Error(err)
	}
	if status != VolumeDetaching {
		t.Error("Expected volume to start detaching")
	}
}

func TestCreateSnapshotIntegration(t *testing.T) {
	if !*integration {
		t.Skip("Integration tests not enabled")
		return
	}

	client := &http.Client{Transport: newLoggingTransport()}
	sr := NewSignedRequester(client, *endpoint, DefaultSigner)

	testSnapshot, err := CreateSnapshot(sr, *volume, "integration-test")
	if err != nil {
		t.Error(err)
	}

	t.Log(testSnapshot)
}

func TestSnapshotByIdIntegration(t *testing.T) {
	if !*integration {
		t.Skip("Integration tests not enabled")
		return
	}

	client := &http.Client{Transport: newLoggingTransport()}
	sr := NewSignedRequester(client, *endpoint, DefaultSigner)
	snap, err := SnapshotById(sr, *snapshot)
	if err != nil {
		t.Error(err)
	}
	if snap.Status != SnapshotCompleted {
		t.Error("Expected snapshot status to be", SnapshotCompleted)
	}

	t.Log(snap)
}

func TestDeleteSnapshotIntegration(t *testing.T) {
	if !*integration {
		t.Skip("Integration tests not enabled")
		return
	}

	client := &http.Client{Transport: newLoggingTransport()}
	sr := NewSignedRequester(client, *endpoint, DefaultSigner)
	if err := DeleteSnapshot(sr, *snapshot); err != nil {
		t.Error(err)
	}
}

func TestDeleteVolumeIntegration(t *testing.T) {
	if !*integration {
		t.Skip("Integration tests not enabled")
		return
	}

	client := &http.Client{Transport: newLoggingTransport()}
	sr := NewSignedRequester(client, *endpoint, DefaultSigner)
	if err := DeleteVolume(sr, *volume); err != nil {
		t.Error(err)
	}
}

func TestAssociateAddressIntegration(t *testing.T) {
	if !*integration {
		t.Skip("Integration tests not enabled")
		return
	}

	client := &http.Client{Transport: newLoggingTransport()}
	sr := NewSignedRequester(client, *endpoint, DefaultSigner)
	if err := AssociateAddress(sr, *instance, *eip); err != nil {
		t.Error(err)
	}
}
