package aws

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
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
