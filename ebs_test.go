package aws

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestVolumeByName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a := "DescribeVolumes"; r.URL.Query().Get("Action") != a {
			t.Errorf("Expected Action to be %s", a)
		}
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<DescribeVolumesResponse xmlns="http://ec2.amazonaws.com/doc/2014-05-01/">
    <requestId>8f0ea6b0-8a7c-40f1-bc41-cd2cf2d887d5</requestId>
    <volumeSet>
        <item>
            <volumeId>vol-72d8f579</volumeId>
            <size>1</size>
            <snapshotId/>
            <availabilityZone>eu-west-1a</availabilityZone>
            <status>available</status>
            <createTime>2014-10-03T15:18:42.354Z</createTime>
            <attachmentSet/>
            <tagSet>
                <item>
                    <key>Name</key>
                    <value>test</value>
                </item>
                <item>
                    <key>Stack</key>
                    <value>joonix-cluster</value>
                </item>
            </tagSet>
            <volumeType>standard</volumeType>
            <encrypted>false</encrypted>
        </item>
    </volumeSet>
</DescribeVolumesResponse>`)
	}))
	defer ts.Close()

	ebs, err := NewEbsClient(http.DefaultClient, ts.URL, defaultSigner)
	if err != nil {
		t.Error(err)
	}

	vol, err := ebs.VolumesByTags([]TagItem{TagItem{"Name", "test"}})
	if err != nil {
		t.Error(err)
	}
	if len(vol) != 1 {
		t.Error("Expected exactly one volume")
	}

	t.Log(vol[0])
	if vol[0].Id != "vol-72d8f579" {
		t.Error("Expected to have the volume id set")
	}
	if vol[0].AvailabilityZone != "eu-west-1a" {
		t.Error("Expected availability zone")
	}
	if len(vol[0].TagSet.Items) != 2 {
		t.Error("Error expected correct amount of tags")
	}
	if vol[0].TagSet.Items[0].Key != "Name" {
		t.Error("Name tag missing")
	}
	if vol[0].TagSet.Items[0].Value != "test" {
		t.Error("Name tag value incorrect")
	}
}

func TestCreateNew(t *testing.T) {
	calls := []string{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		replies := map[string]string{
			"CreateVolume": `<?xml version="1.0" encoding="UTF-8"?>
<CreateVolumeResponse xmlns="http://ec2.amazonaws.com/doc/2014-05-01/">
    <requestId>6bcd00ed-1518-4f63-b757-c79e6b62c031</requestId>
    <volumeId>vol-842b078f</volumeId>
    <size>1</size>
    <snapshotId/>
    <availabilityZone>eu-west-1a</availabilityZone>
    <status>creating</status>
    <createTime>2014-10-04T16:30:35.740Z</createTime>
    <volumeType>standard</volumeType>
    <encrypted>false</encrypted>
</CreateVolumeResponse>`,
			"CreateTags": `<?xml version="1.0" encoding="UTF-8"?>
<CreateTagsResponse xmlns="http://ec2.amazonaws.com/doc/2014-05-01/">
    <requestId>027f0d56-349f-43ff-a937-a2d7cd4c5e0a</requestId>
    <return>true</return>
</CreateTagsResponse>`}

		action := r.URL.Query().Get("Action")
		if reply, ok := replies[action]; !ok {
			t.Errorf("Invalid action '%s'")
		} else {
			fmt.Fprint(w, reply)
			calls = append(calls, action)
		}
	}))
	defer ts.Close()

	ebs, err := NewEbsClient(http.DefaultClient, ts.URL, defaultSigner)
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
	if id := "vol-842b078f"; testVolume.Id != id {
		t.Error("Expected id to be %s", id)
	}
	if len(calls) != 2 {
		t.Error("Expected exactly 2 requests")
	}
}

func TestCreateNewPiops(t *testing.T) {
	called := make(chan bool)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if vt := r.URL.Query().Get("VolumeType"); vt != "io1" {
			t.Error("Expected VolumeType to be io1")
		}
		close(called)
	}))
	defer ts.Close()

	ebs, err := NewEbsClient(http.DefaultClient, ts.URL, defaultSigner)
	if err != nil {
		t.Error(err)
	}
	_, err = ebs.CreateVolume(1, 1000, false, "eu-west-1a", "", []TagItem{})
	if err == nil {
		t.Error("Was expecting an error")
	} else if err.Error() != "Provisioned IOPS volumes are only available as SSD" {
		t.Error("Unexpected error", err)
	}

	_, err = ebs.CreateVolume(1, 1000, true, "eu-west-1a", "", []TagItem{})
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Error("No request was made")
	}
}

func TestAttachVolume(t *testing.T) {
	t.Skip("Not implemented yet")
}
