package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/joonix/aws"
	"log"
	"net/http"
	"os"
	"time"
)

var sslClient *http.Client

func init() {
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(pemCerts)
	sslClient = &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}}
}

func detachEbs(c *cli.Context) {
	client, err := aws.NewEbsClient(sslClient, c.GlobalString("endpoint"), nil)
	if err != nil {
		log.Fatal(err)
	}

	tags := []aws.TagItem{
		aws.TagItem{"Name", c.String("name")},
	}
	vols, err := client.VolumesByTags(tags)
	if err != nil {
		log.Fatalf("Not able to find the volume by name %s: %s", c.String("name"), err)
	}

	if len(vols) != 1 {
		log.Fatalf("Expected exactly one volume by the name %s", c.String("name"))
	}
	status, err := client.DetachVolume(vols[0].Id)
	if err != nil {
		log.Fatalf("Could not detach volume: %s", err)
	}
	log.Println(status)
}

func attachEbs(c *cli.Context) {
	client, err := aws.NewEbsClient(sslClient, c.GlobalString("endpoint"), nil)
	if err != nil {
		log.Fatal(err)
	}

	// See if a volume already exists and is in the same AZ as us
	tags := []aws.TagItem{
		aws.TagItem{"Name", c.String("name")},
	}
	vols, err := client.VolumesByTags(tags)
	if err != nil {
		log.Fatalln(err)
	}
	if len(vols) > 1 {
		log.Fatalf("More than one volume exist with the name %s", c.String("name"))
	}

	// TODO: Try getting from EC2 metadata service if empty
	instanceAz := c.String("az")
	instanceId := c.String("instance")

	snapshot := c.String("snapshot")

	var volume *aws.EbsVolume
	if len(vols) == 1 {
		if vols[0].AvailabilityZone != instanceAz {
			// Volume needs to be migrated to the same AZ as the instance by using a snapshot.
			snap, err := client.CreateSnapshot(vols[0].Id, "migrate_zone")
			if err != nil {
				log.Fatalf("Error creating snapshot: %s", err)
			}
			// TODO: Prune old migration snapshots
			snapshot = snap.Id

			wait := make(chan bool)
			go func() {
				defer close(wait)
				for {
					time.Sleep(time.Second)
					snap, err := client.SnapshotById(snapshot)
					if err != nil {
						log.Fatalf("Could not update snapshot status for %s", snapshot)
					}
					if snap.Status == aws.SnapshotCompleted {
						return
					}
				}
			}()
			select {
			case <-time.After(30 * time.Second):
				log.Fatalf("Timed out waiting for snapshot %s to complete", snapshot)
			case <-wait:
				log.Println("Created snapshot", snapshot)
				if err := client.DeleteVolume(vols[0].Id); err != nil {
					log.Printf("WARNING: Was not able to delete old volume %s\n", vols[0].Id)
				}
			}
		} else {
			// Same AZ, we can attach the already existing volume.
			volume = &vols[0]
			log.Println("Re-Used volume from same AZ", volume.Id)
		}
	}

	if volume == nil {
		var err error
		if volume, err = client.CreateVolume(uint(c.Int("size")), uint(c.Int("piops")), c.Bool("ssd"), instanceAz, snapshot, tags); err != nil {
			log.Fatal(err)
		} else {
			wait := make(chan bool)
			go func() {
				defer close(wait)
				for {
					time.Sleep(time.Second)
					if volume, err = client.VolumeById(volume.Id); err != nil {
						log.Fatalf("Could not update volume status for %s", volume.Id)
					}
					if volume.Status == aws.VolumeAvailable {
						return
					}
				}
			}()

			select {
			case <-time.After(30 * time.Second):
				log.Fatalf("Timed out waiting for volume %s to become available", volume.Id)
			case <-wait:
				log.Println("Created volume", volume.Id)
			}
		}
	}

	// Check if we already have this volume attached to this instance
	if volume.Status == aws.VolumeInUse {
		attachment := volume.AttachmentSet.Items[0]
		if attachment.InstanceId == instanceId {
			fmt.Println(attachment.Device)
			return
		}
	}

	// Finally attach volume and print path
	path, err := client.AttachVolume(volume.Id, instanceId)
	if err != nil {
		log.Fatalf("Could not attach volume: %s\n", err)
	}
	fmt.Println(path)
}

func main() {
	log.SetPrefix("")

	app := cli.NewApp()
	app.Name = "joonix-cluster"
	app.Usage = "Joonix AWS cluster administration"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "endpoint",
			Usage: "The AWS endpoint to use",
			Value: "https://ec2.eu-west-1.amazonaws.com",
		},
	}
	app.Commands = []cli.Command{
		{
			Name:  "ebs",
			Usage: "options for Elastic Block Storage",
			Subcommands: []cli.Command{
				{
					Name:  "attach",
					Usage: "attach a new volume or create one if matching name doesn't exist",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:   "name",
							Usage:  "name tag of volume to attach",
							EnvVar: "EBS_ATTACH_NAME",
						},
						cli.IntFlag{
							Name:   "size",
							Usage:  "size of volume in GiB",
							EnvVar: "EBS_ATTACH_SIZE",
							Value:  10,
						},
						cli.BoolFlag{
							Name:   "ssd",
							Usage:  "SSD or not",
							EnvVar: "EBS_ATTACH_SSD",
						},
						cli.IntFlag{
							Name:   "piops",
							Usage:  "Number of provisioned IOPS to request",
							EnvVar: "EBS_ATTACH_PIOPS",
							Value:  0,
						},
						cli.StringFlag{
							Name:   "snapshot",
							Usage:  "Snapshot to use if the volume does not already exist",
							EnvVar: "EBS_ATTACH_SNAPSHOT",
						},
						cli.StringFlag{
							Name:   "instance",
							Usage:  "Instance id to attach to",
							EnvVar: "EBS_ATTACH_INSTANCE",
						},
						cli.StringFlag{
							Name:   "az",
							Usage:  "Availability Zone in which the instance is running",
							EnvVar: "EBS_ATTACH_AZ",
						},
					},
					Action: attachEbs,
				},
				{
					Name:  "detach",
					Usage: "detach a volume from an instance",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:   "name",
							Usage:  "name tag of volume to detach",
							EnvVar: "EBS_DETACH_NAME",
						},
					},
					Action: detachEbs,
				},
			},
		},
	}

	app.Run(os.Args)
}
