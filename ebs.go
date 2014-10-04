package aws

import (
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/smartystreets/go-aws-auth"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

var defaultSigner = RequestSignerFunc(func(r *http.Request) {
	awsauth.Sign(r)
})

// RequestSigner describes how to Sign requests before sending them to Amazon Web Services API.
type RequestSigner interface {
	Sign(*http.Request)
}

// RequestSignerFunc wraps a function to implement the RequestSigner interface.
type RequestSignerFunc func(*http.Request)

func (f RequestSignerFunc) Sign(r *http.Request) {
	f(r)
}

type TagItem struct {
	Key   string `xml:"key"`
	Value string `xml:"value"`
}

type EbsVolume struct {
	Id               string    `xml:"volumeId"`
	AvailabilityZone string    `xml:"availabilityZone"`
	Status           string    `xml:"status"`
	CreatedAt        time.Time `xml:"createTime"`
	TagSet           struct {
		Items []TagItem `xml:"item"`
	} `xml:"tagSet"`
}

type EbsVolumeSet struct {
	VolumeSet struct {
		Items []EbsVolume `xml:"item"`
	} `xml:"volumeSet"`
}

// EbsClient handles the actions related to Elastic Block Storage.
type EbsClient struct {
	client   *http.Client
	endpoint string
	signer   RequestSigner
}

func NewEbsClient(client *http.Client, endpoint string, signer RequestSigner) (*EbsClient, error) {
	ebs := new(EbsClient)

	if endpoint == "" {
		ebs.endpoint = "https://ec2.amazonaws.com"
	} else {
		if _, err := url.Parse(endpoint); err != nil {
			return nil, err
		}
		ebs.endpoint = endpoint
	}
	if client == nil {
		ebs.client = http.DefaultClient
	} else {
		ebs.client = client
	}
	if signer == nil {
		ebs.signer = defaultSigner
	} else {
		ebs.signer = signer

	}

	return ebs, nil
}

// signedRequest applies the signature the the request using provided RequestSigner.
func (ebs *EbsClient) signedRequest(req *http.Request) (*http.Response, error) {
	// Version param is required for Amazon to understand the request.
	values := req.URL.Query()
	values.Add("Version", "2014-05-01")
	req.URL.RawQuery = values.Encode()

	ebs.signer.Sign(req)
	return ebs.client.Do(req)
}

// VolumesByTags will return list of volumes that matches the specified tags.
func (ebs *EbsClient) VolumesByTags(tags []TagItem) ([]EbsVolume, error) {
	req, err := http.NewRequest("GET", ebs.endpoint, nil)
	if err != nil {
		return nil, err
	}
	values := req.URL.Query()
	values.Add("Action", "DescribeVolumes")

	for n, tag := range tags {
		values.Add(fmt.Sprintf("Filter.%d.Name", n+1), "tag:"+tag.Key)
		values.Add(fmt.Sprintf("Filter.%d.Value", n+1), tag.Value)
	}
	req.URL.RawQuery = values.Encode()

	res, err := ebs.signedRequest(req)
	if err != nil {
		return nil, err
	}

	b, _ := ioutil.ReadAll(res.Body)
	if res.StatusCode != 200 {
		return nil, errors.New(string(b))
	}

	set := new(EbsVolumeSet)
	if err := xml.Unmarshal(b, set); err != nil {
		return nil, err
	}

	return set.VolumeSet.Items, nil
}

// CreateVolume creates a new volume using specified properties.
func (ebs *EbsClient) CreateVolume(size uint, piops uint, ssd bool, az string, snapshot string, tags []TagItem) (*EbsVolume, error) {
	req, err := http.NewRequest("GET", ebs.endpoint, nil)
	if err != nil {
		return nil, err
	}
	values := req.URL.Query()
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

	req.URL.RawQuery = values.Encode()

	res, err := ebs.signedRequest(req)
	if err != nil {
		return nil, err
	}

	b, _ := ioutil.ReadAll(res.Body)
	if res.StatusCode != 200 {
		return nil, errors.New(string(b))
	}

	vol := new(EbsVolume)
	if err := xml.Unmarshal(b, vol); err != nil {
		return nil, err
	}

	// Volume is created, but creating tags is a separate request
	if len(tags) > 0 {
		if err = ebs.tagResource(vol.Id, tags); err != nil {
			return nil, err
		}
	}
	return vol, nil
}

func (ebs *EbsClient) tagResource(id string, tags []TagItem) error {
	req, err := http.NewRequest("GET", ebs.endpoint, nil)
	if err != nil {
		return err
	}
	values := req.URL.Query()
	values.Add("Action", "CreateTags")
	values.Add("ResourceId.1", id)
	for n, tag := range tags {
		values.Add(fmt.Sprintf("Tag.%d.Key", n+1), tag.Key)
		values.Add(fmt.Sprintf("Tag.%d.Value", n+1), tag.Value)
	}
	req.URL.RawQuery = values.Encode()

	res, err := ebs.signedRequest(req)
	if err != nil {
		return err
	}

	if res.StatusCode != 200 {
		b, _ := ioutil.ReadAll(res.Body)
		return errors.New(string(b))
	}

	return nil
}
