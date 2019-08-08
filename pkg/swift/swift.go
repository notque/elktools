package swift

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/majewsky/schwift"
	"github.com/majewsky/schwift/gopherschwift"
)

type Swift struct {
	container *schwift.Container
}

func NewSwift(container string) (*Swift, error) {
	if container == "" {
		return nil, errors.New("Container name required")
	}
	fmt.Printf("os domainname: %s\n", os.Getenv("OS_DOMAIN_NAME"))
	authOptions, err := openstack.AuthOptionsFromEnv()
	if err != nil {
		return nil, err
	}
	if authOptions.DomainName == "" {
		return nil, errors.New("DomainName not set")
	}

	if os.Getenv("OS_PROJECT_DOMAIN_NAME") != "" {
		//fmt.Printf("projectname: %s\n", os.Getenv("OS_PROJECT_NAME"))
		//fmt.Printf("os projectdomainname: %s\n", os.Getenv("OS_PROJECT_DOMAIN_NAME"))
		authOptions.Scope = &gophercloud.AuthScope{
			ProjectName: os.Getenv("OS_PROJECT_NAME"),
			DomainName:  os.Getenv("OS_PROJECT_DOMAIN_NAME"),
		}
	}

	authOptions.AllowReauth = true
	provider, err := openstack.AuthenticatedClient(authOptions)
	if err != nil {
		fmt.Print("Failed to AuthenticatedClient\n")
		return nil, err
	}
	client, err := openstack.NewObjectStorageV1(provider, gophercloud.EndpointOpts{})
	if err != nil {
		fmt.Print("Failed to NewObjectStorageV1\n")
		return nil, err
	}

	account, err := gopherschwift.Wrap(client, nil)
	if err != nil {
		fmt.Print("Failed to gopherschwift.Wrap\n")
		return nil, err
	}
	c, err := account.Container(container).EnsureExists()
	if err != nil {
		fmt.Print("Failed to Container Exist\n")
		return nil, err
	}
	return &Swift{container: c}, nil
}

func Get(s *Swift, key string) (*url.URL, bool, error) {
	key = strings.TrimLeft(key, "/")
	object := s.container.Object(key)
	exists, err := object.Exists()
	if err != nil {
		return nil, false, err
	}
	if exists {
		objectURL, err := object.URL()
		if err != nil {
			return nil, false, err
		}
		u, err := url.Parse(objectURL)
		return u, true, err
	}
	return nil, false, nil
}

func Store(s *Swift, key string, tmpfile io.ReadSeeker) (*url.URL, error) {

	key = strings.TrimLeft(key, "/")

	object := s.container.Object(key)

	if err := object.Upload(tmpfile, nil, nil); err != nil {
		return nil, err
	}
	objectURL, err := object.URL()
	if err != nil {
		return nil, err
	}
	return url.Parse(objectURL)

}

func ListContents(s *Swift) error {
	iter := s.container.Objects()

	iter.Prefix = "events/"
	objects, err := iter.Collect()
	if err != nil {
		return err
	}

	for _, item := range objects {
		fmt.Printf("Objects: %s\n", item.FullName())
	}

	return nil
}

func ContentsAsString(s *Swift) (string, error) {
	iter := s.container.Objects()

	iter.Prefix = "events/"
	objects, err := iter.Collect()

	if err != nil {
		return "", err
	}

	for _, item := range objects {
		str, err := item.Download(nil).AsString()
		if err != nil {
			return "", err
		}
		//fmt.Printf("Events: %s\n", str)

		return str, nil // One file for testing
	}

	return "", nil
}
