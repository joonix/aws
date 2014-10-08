package aws

import (
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type DeviceMapping struct {
	Device string `xml:"deviceName"`
	Info   struct {
		Id                  string            `xml:"volumeId"`
		Status              AttachementStatus `xml:"status"`
		AttachAt            time.Time         `xml:"attachTime"`
		DeleteOnTermination bool              `xml:"deleteOnTermination"`
	} `xml:"ebs"`
}

type AttachementStatus string

var (
	VolumeAttaching AttachementStatus = "attaching"
	VolumeAttached  AttachementStatus = "attached"
	VolumeDetaching AttachementStatus = "detaching"
	VolumeDetached  AttachementStatus = "detached"
)

func (s AttachementStatus) String() string {
	return string(s)
}

type EbsVolumeAttachementResponse struct {
	InstanceId string            `xml:"instanceId"`
	VolumeId   string            `xml:"volumeId"`
	Status     AttachementStatus `xml:"status"`
	Device     string            `xml:"device"`
}

type EbsVolume struct {
	Id               string       `xml:"volumeId"`
	AvailabilityZone string       `xml:"availabilityZone"`
	Status           VolumeStatus `xml:"status"`
	CreatedAt        time.Time    `xml:"createTime"`
	AttachmentSet    struct {
		Items []EbsVolumeAttachementResponse `xml:"item"`
	} `xml:"attachmentSet"`
	TagSet struct {
		Items []TagItem `xml:"item"`
	} `xml:"tagSet"`
}

type VolumeStatus string

//  creating | available | in-use | deleting | deleted | error
var (
	VolumeInUse     VolumeStatus = "in-use"
	VolumeCreating  VolumeStatus = "creating"
	VolumeAvailable VolumeStatus = "available"
	VolumeDeleting  VolumeStatus = "deleting"
	VolumeDeleted   VolumeStatus = "deleted"
	VolumeError     VolumeStatus = "error"
)

func (v VolumeStatus) String() string {
	return string(v)
}

type EbsVolumeSet struct {
	VolumeSet struct {
		Items []EbsVolume `xml:"item"`
	} `xml:"volumeSet"`
}

type SnapshotStatus string

var (
	SnapshotCompleted SnapshotStatus = "completed"
	SnapshotPending   SnapshotStatus = "pending"
	SnapshotError     SnapshotStatus = "error"
)

func (s SnapshotStatus) String() string {
	return string(s)
}

type EbsSnapshot struct {
	Id          string         `xml:"snapshotId"`
	VolumeId    string         `xml:"volumeId"`
	Status      SnapshotStatus `xml:"status"`
	Description string         `xml:"description"`
}

type EbsSnapshotSet struct {
	SnapshotSet struct {
		Items []EbsSnapshot `xml:"item"`
	} `xml:"snapshotSet"`
}

// VolumesByTags will return list of volumes that matches the specified tags.
func VolumesByTags(sr SignedRequester, tags []TagItem) ([]EbsVolume, error) {
	values := make(url.Values)
	values.Add("Action", "DescribeVolumes")

	for n, tag := range tags {
		values.Add(fmt.Sprintf("Filter.%d.Name", n+1), "tag:"+tag.Key)
		values.Add(fmt.Sprintf("Filter.%d.Value", n+1), tag.Value)
	}

	b, err := sr.SignedRequest(values)
	if err != nil {
		return nil, err
	}

	set := new(EbsVolumeSet)
	if err := xml.Unmarshal(b, set); err != nil {
		return nil, err
	}

	return set.VolumeSet.Items, nil
}

// VolumeById will return the volume that matches the specified id.
func VolumeById(sr SignedRequester, id string) (*EbsVolume, error) {
	values := make(url.Values)
	values.Add("Action", "DescribeVolumes")
	values.Add("VolumeId.1", id)

	b, err := sr.SignedRequest(values)
	if err != nil {
		return nil, err
	}

	set := new(EbsVolumeSet)
	if err := xml.Unmarshal(b, set); err != nil {
		return nil, err
	}

	if len(set.VolumeSet.Items) != 1 {
		return nil, errors.New("Could not find the specified volume")
	}
	return &set.VolumeSet.Items[0], nil
}

// CreateVolume creates a new volume using specified properties.
func CreateVolume(sr SignedRequester, size uint, piops uint, ssd bool, az, snapshot string, tags []TagItem) (*EbsVolume, error) {
	values := make(url.Values)
	values.Add("Action", "CreateVolume")
	values.Add("Size", strconv.Itoa(int(size)))
	values.Add("AvailabilityZone", az)

	if snapshot != "" {
		values.Add("SnapshotId", snapshot)
	}
	if piops > 0 {
		if !ssd {
			return nil, errors.New("Provisioned IOPS volumes are only available as SSD")
		}
		values.Add("VolumeType", "io1")
		values.Add("Iops", strconv.Itoa(int(piops)))
	} else if ssd {
		values.Add("VolumeType", "gp2")
	} else {
		values.Add("VolumeType", "standard")
	}

	b, err := sr.SignedRequest(values)
	if err != nil {
		return nil, err
	}

	vol := new(EbsVolume)
	if err := xml.Unmarshal(b, vol); err != nil {
		return nil, err
	}

	// Volume is created, but creating tags is a separate request
	if len(tags) > 0 {
		if err = TagResource(sr, vol.Id, tags); err != nil {
			return nil, err
		}
	}
	return vol, nil
}

func DeleteVolume(sr SignedRequester, id string) error {
	values := make(url.Values)
	values.Add("Action", "DeleteVolume")
	values.Add("VolumeId", id)

	if _, err := sr.SignedRequest(values); err != nil {
		return err
	}

	return nil
}

func TagResource(sr SignedRequester, id string, tags []TagItem) error {
	values := make(url.Values)
	values.Add("Action", "CreateTags")
	values.Add("ResourceId.1", id)

	for n, tag := range tags {
		values.Add(fmt.Sprintf("Tag.%d.Key", n+1), tag.Key)
		values.Add(fmt.Sprintf("Tag.%d.Value", n+1), tag.Value)
	}

	if _, err := sr.SignedRequest(values); err != nil {
		return err
	}

	return nil
}

func AttachVolume(sr SignedRequester, id, instance string) (device string, err error) {
	var mapping []DeviceMapping
	mapping, err = GetBlockDeviceMapping(sr, instance)
	if err != nil {
		return
	}

	device = "/dev/sdf"
	for _, item := range mapping {
		if !strings.HasPrefix(item.Device, "/dev/sd") {
			continue
		}
		// Put our device one step ahead of anyone else by naively shifting the last byte
		if item.Device >= device {
			b := []byte(item.Device)
			device = string(append(b[:len(b)-1], b[len(b)-1]+1))
		}
	}

	values := make(url.Values)
	values.Add("Action", "AttachVolume")
	values.Add("InstanceId", instance)
	values.Add("VolumeId", id)
	values.Add("Device", device)

	_, err = sr.SignedRequest(values)
	return
}

func DetachVolume(sr SignedRequester, id string) (AttachementStatus, error) {
	values := make(url.Values)
	values.Add("Action", "DetachVolume")
	values.Add("VolumeId", id)

	b, err := sr.SignedRequest(values)
	if err != nil {
		return AttachementStatus(""), err
	}

	volres := new(EbsVolumeAttachementResponse)
	err = xml.Unmarshal(b, volres)

	return volres.Status, err
}

func GetBlockDeviceMapping(sr SignedRequester, instance string) ([]DeviceMapping, error) {
	values := make(url.Values)
	values.Add("Action", "DescribeInstanceAttribute")
	values.Add("InstanceId", instance)
	values.Add("Attribute", "blockDeviceMapping")

	b, err := sr.SignedRequest(values)
	if err != nil {
		return nil, err
	}

	m := &struct {
		Mappings struct {
			Item []DeviceMapping `xml:"item"`
		} `xml:"blockDeviceMapping"`
	}{}
	if err := xml.Unmarshal(b, &m); err != nil {
		return nil, err
	}

	return m.Mappings.Item, nil
}

func CreateSnapshot(sr SignedRequester, volume, description string) (*EbsSnapshot, error) {
	values := make(url.Values)
	values.Add("Action", "CreateSnapshot")
	values.Add("Description", description)
	values.Add("VolumeId", volume)

	b, err := sr.SignedRequest(values)
	if err != nil {
		return nil, err
	}

	snap := new(EbsSnapshot)
	if err := xml.Unmarshal(b, snap); err != nil {
		return nil, err
	}

	return snap, nil
}

func SnapshotById(sr SignedRequester, id string) (*EbsSnapshot, error) {
	values := make(url.Values)
	values.Add("Action", "DescribeSnapshots")
	values.Add("SnapshotId.1", id)

	b, err := sr.SignedRequest(values)
	if err != nil {
		return nil, err
	}

	snapset := new(EbsSnapshotSet)
	if err := xml.Unmarshal(b, snapset); err != nil {
		return nil, err
	}

	if len(snapset.SnapshotSet.Items) != 1 {
		return nil, errors.New("Could not find the specified snapshot")
	}
	return &snapset.SnapshotSet.Items[0], nil
}

func DeleteSnapshot(sr SignedRequester, id string) error {
	values := make(url.Values)
	values.Add("Action", "DeleteSnapshot")
	values.Add("SnapshotId", id)

	if _, err := sr.SignedRequest(values); err != nil {
		return err
	}

	return nil
}
