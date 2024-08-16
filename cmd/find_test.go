package cmd

import (
	"fmt"
	"testing"
	"time"

	"github.com/segmentio/chamber/v3/store"
	"github.com/stretchr/testify/assert"
)

func TestFindFunctions(t *testing.T) {
	tests := []struct {
		name   string
		params string
		output string
	}{
		{name: "service1", params: "/service1/key_one", output: "service1"},
		{name: "service2", params: "/service2/subService/key_two", output: "service2/subService"},
		{name: "service3", params: "/service3/subService/subSubService/key_three", output: "service3/subService/subSubService"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := path(test.params)
			assert.Equal(t, test.output, result)
		})
	}

	keyMatchTests := []struct {
		name       string
		services   []string
		searchTerm string
		output     []store.SecretId
	}{
		{
			name: "findNoMatches",
			services: []string{
				"/service1/launch_darkly_key",
				"/service2/s3_bucket_base",
				"/service3/slack_token",
			},
			searchTerm: "s3_bucket",
			output:     []store.SecretId{},
		},
		{
			name: "findSomeMatches",
			services: []string{
				"/service1/s3_bucket",
				"/service2/s3_bucket_base",
				"/service3/s3_bucket",
			},
			searchTerm: "s3_bucket",
			output: []store.SecretId{
				{Service: "service1", Key: "s3_bucket"},
				{Service: "service3", Key: "s3_bucket"},
			},
		},
		{
			name: "findEverythingMatches",
			services: []string{
				"/service1/s3_bucket",
				"/service2/s3_bucket",
				"/service3/s3_bucket",
			},
			searchTerm: "s3_bucket",
			output: []store.SecretId{
				{Service: "service1", Key: "s3_bucket"},
				{Service: "service2", Key: "s3_bucket"},
				{Service: "service3", Key: "s3_bucket"},
			},
		},
	}

	for _, test := range keyMatchTests {
		t.Run(test.name, func(t *testing.T) {
			result := findKeyMatch(test.services, test.searchTerm)
			fmt.Println(result)
			assert.Equal(t, test.output, result)
		})
	}

	valueDarklyToken := "1@m@Pr3t3ndL@unchD@rkl3yK3y"
	valueSlackToken := "1@m@Pr3t3ndSlackToken"
	valueGoodS3Bucket := "s3://this_bucket"
	valueBadS3Bucket := "s3://not_your_bucket"

	valueMatchTests := []struct {
		name       string
		secrets    []store.Secret
		searchTerm string
		output     []store.SecretId
	}{
		{
			name: "findNoMatches",
			secrets: []store.Secret{
				{
					Value: &valueDarklyToken,
					Meta: store.SecretMetadata{
						Created:   time.Now(),
						CreatedBy: "no one",
						Version:   0,
						Key:       "/service1/launch_darkly_key",
					},
				},
				{
					Value: &valueSlackToken,
					Meta: store.SecretMetadata{
						Created:   time.Now(),
						CreatedBy: "no one",
						Version:   0,
						Key:       "/service1/slack_token",
					},
				},
				{
					Value: &valueBadS3Bucket,
					Meta: store.SecretMetadata{
						Created:   time.Now(),
						CreatedBy: "no one",
						Version:   0,
						Key:       "/service1/s3_bucket",
					},
				},
			},
			searchTerm: "s3://this_bucket",
			output:     []store.SecretId{},
		},
		{
			"findSomeMatches",
			[]store.Secret{
				{
					Value: &valueDarklyToken,
					Meta: store.SecretMetadata{
						Created:   time.Now(),
						CreatedBy: "no one",
						Version:   0,
						Key:       "/service1/launch_darkly_key",
					},
				},
				{
					Value: &valueGoodS3Bucket,
					Meta: store.SecretMetadata{
						Created:   time.Now(),
						CreatedBy: "no one",
						Version:   0,
						Key:       "/service1/s3_bucket_name",
					},
				},
				{
					Value: &valueGoodS3Bucket,
					Meta: store.SecretMetadata{
						Created:   time.Now(),
						CreatedBy: "no one",
						Version:   0,
						Key:       "/service1/s3_bucket",
					},
				},
			},
			"s3://this_bucket",
			[]store.SecretId{
				{
					Service: "service1",
					Key:     "s3_bucket_name",
				},
				{
					Service: "service1",
					Key:     "s3_bucket",
				},
			},
		},
		{
			"findEverythingMatches",
			[]store.Secret{
				{
					Value: &valueGoodS3Bucket,
					Meta: store.SecretMetadata{
						Created:   time.Now(),
						CreatedBy: "no one",
						Version:   0,
						Key:       "/service1/s3_bucket_base",
					},
				},
				{
					Value: &valueGoodS3Bucket,
					Meta: store.SecretMetadata{
						Created:   time.Now(),
						CreatedBy: "no one",
						Version:   0,
						Key:       "/service1/s3_bucket_name",
					},
				},
				{
					Value: &valueGoodS3Bucket,
					Meta: store.SecretMetadata{
						Created:   time.Now(),
						CreatedBy: "no one",
						Version:   0,
						Key:       "/service1/s3_bucket",
					},
				},
			},
			"s3://this_bucket",
			[]store.SecretId{
				{
					Service: "service1",
					Key:     "s3_bucket_base",
				},
				{
					Service: "service1",
					Key:     "s3_bucket_name",
				},
				{
					Service: "service1",
					Key:     "s3_bucket",
				},
			},
		},
	}

	for _, test := range valueMatchTests {
		t.Run(test.name, func(t *testing.T) {
			result := findValueMatch(test.secrets, test.searchTerm)
			assert.Equal(t, test.output, result)
		})
	}

}
