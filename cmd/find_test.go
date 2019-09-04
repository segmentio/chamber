package cmd

import (
	"fmt"
	"testing"
	"time"

	"github.com/segmentio/chamber/store"
	"github.com/stretchr/testify/assert"
)

func TestFindFunctions(t *testing.T) {
	tests := []struct {
		name   string
		params string
		output string
	}{
		{
			"service1",
			"/service1/key_one",
			"service1",
		},
		{
			"service2",
			"/service2/subService/key_two",
			"service2/subService",
		},
		{
			"service3",
			"/service3/subService/subSubService/key_three",
			"service3/subService/subSubService",
		},
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
			"findNoMatches",
			[]string{"/service1/launch_darkly_key",
				"/service2/s3_bucket_base",
				"/service3/slack_token",
			},
			"s3_bucket",
			[]store.SecretId{},
		},
		{
			"findSomeMatches",
			[]string{"/service1/s3_bucket",
				"/service2/s3_bucket_base",
				"/service3/s3_bucket",
			},
			"s3_bucket",
			[]store.SecretId{
				{
					"service1",
					"s3_bucket",
				},
				{
					"service3",
					"s3_bucket",
				},
			},
		},
		{
			"findEverythingMatches",
			[]string{"/service1/s3_bucket",
				"/service2/s3_bucket",
				"/service3/s3_bucket",
			},
			"s3_bucket",
			[]store.SecretId{
				{
					"service1",
					"s3_bucket",
				},
				{
					"service2",
					"s3_bucket",
				},
				{
					"service3",
					"s3_bucket",
				},
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
			"findNoMatches",
			[]store.Secret{
				store.Secret{
					&valueDarklyToken,
					store.SecretMetadata{
						time.Now(),
						"no one",
						0,
						"/service1/launch_darkly_key",
					},
				},
				store.Secret{
					&valueSlackToken,
					store.SecretMetadata{
						time.Now(),
						"no one",
						0,
						"/service1/slack_token",
					},
				},
				store.Secret{
					&valueBadS3Bucket,
					store.SecretMetadata{
						time.Now(),
						"no one",
						0,
						"/service1/s3_bucket",
					},
				},
			},
			"s3://this_bucket",
			[]store.SecretId{},
		},
		{
			"findSomeMatches",
			[]store.Secret{
				store.Secret{
					&valueDarklyToken,
					store.SecretMetadata{
						time.Now(),
						"no one",
						0,
						"/service1/launch_darkly_key",
					},
				},
				store.Secret{
					&valueGoodS3Bucket,
					store.SecretMetadata{
						time.Now(),
						"no one",
						0,
						"/service1/s3_bucket_name",
					},
				},
				store.Secret{
					&valueGoodS3Bucket,
					store.SecretMetadata{
						time.Now(),
						"no one",
						0,
						"/service1/s3_bucket",
					},
				},
			},
			"s3://this_bucket",
			[]store.SecretId{
				{
					"service1",
					"s3_bucket_name",
				},
				{
					"service1",
					"s3_bucket",
				},
			},
		},
		{
			"findEverythingMatches",
			[]store.Secret{
				store.Secret{
					&valueGoodS3Bucket,
					store.SecretMetadata{
						time.Now(),
						"no one",
						0,
						"/service1/s3_bucket_base",
					},
				},
				store.Secret{
					&valueGoodS3Bucket,
					store.SecretMetadata{
						time.Now(),
						"no one",
						0,
						"/service1/s3_bucket_name",
					},
				},
				store.Secret{
					&valueGoodS3Bucket,
					store.SecretMetadata{
						time.Now(),
						"no one",
						0,
						"/service1/s3_bucket",
					},
				},
			},
			"s3://this_bucket",
			[]store.SecretId{
				{
					"service1",
					"s3_bucket_base",
				},
				{
					"service1",
					"s3_bucket_name",
				},
				{
					"service1",
					"s3_bucket",
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
