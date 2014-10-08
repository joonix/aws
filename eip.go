package aws

import (
	"encoding/xml"
	"errors"
	"net/url"
)

type EipAddress struct {
	PublicIp      string `xml:"publicIp"`
	AllocationId  string `xml:"allocationId"`
	InstanceId    string `xml:"instanceId"`
	AssociationId string `xml:"associationId"`
}

func AssociateAddress(sr SignedRequester, instance, ip string) error {
	// Allocattion Id is required for VPC
	eip, err := DescribeAddress(sr, ip)
	if err != nil {
		return err
	}

	values := make(url.Values)
	values.Add("Action", "AssociateAddress")
	values.Add("AllocationId", eip.AllocationId)
	values.Add("InstanceId", instance)
	values.Add("AllowReassociation", "true")

	if _, err := sr.SignedRequest(values); err != nil {
		return err
	}

	return nil
}

func DescribeAddress(sr SignedRequester, ip string) (*EipAddress, error) {
	values := make(url.Values)
	values.Add("Action", "DescribeAddresses")
	values.Add("PublicIp.1", ip)

	res, err := sr.SignedRequest(values)
	if err != nil {
		return nil, err
	}

	addresses := struct {
		AddressesSet struct {
			Items []*EipAddress `xml:"item"`
		} `xml:"addressesSet"`
	}{}

	if err := xml.Unmarshal(res, &addresses); err != nil {
		return nil, err
	}

	if len(addresses.AddressesSet.Items) != 1 {
		return nil, errors.New("Could not find the address")
	}
	return addresses.AddressesSet.Items[0], nil
}
