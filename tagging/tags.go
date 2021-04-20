package tagging

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

var (
	ArgTags            []string
	TagsFilePath       string
	tags               map[string]string
	tagsString         string
	ssmTags            []*ssm.Tag
	secretsManagerTags []*secretsmanager.Tag
)

func EnsureTagsLoaded() error {
	if tags == nil {
		tags = make(map[string]string)
		if err := loadTagsFile(); err != nil {
			return err
		}
		if err := loadArgumentTags(); err != nil {
			return err
		}
	}
	return nil
}

func loadArgumentTags() error {
	for _, tagString := range ArgTags {
		tagItem := strings.SplitN(tagString, "=", 2)

		if len(tagItem) != 2 {
			return errors.New(fmt.Sprintf("tag argument %s is not in a key=value format", tagString))
		}

		tags[tagItem[0]] = tagItem[1]
	}
	return nil
}

func loadTagsFile() error {
	if TagsFilePath != "" {
		var tagsFileIn io.Reader
		var err error

		if TagsFilePath == "-" {
			tagsFileIn = os.Stdin
		} else {
			tagsFileIn, err = os.Open(TagsFilePath)
			if err != nil {
				return errors.Wrap(err, "Failed to open tags file")
			}
		}

		decoder := yaml.NewDecoder(tagsFileIn)
		if err := decoder.Decode(&tags); err != nil {
			return errors.Wrap(err, "Failed to decode tags file input as json")
		}
	}
	return nil
}

func GetTagsString() string {
	if tagsString == "" {
		if len(tags) != 0 {
			var tag_buffer []string
			for k, v := range tags {
				tag_buffer = append(tag_buffer, fmt.Sprintf("%s=%s", url.QueryEscape(k), url.QueryEscape(v)))
			}
			tagsString = strings.Join(tag_buffer[:], "&")
		}
	}

	return tagsString
}

func GetSSMtags() []*ssm.Tag {
	if len(ssmTags) == 0 {
		if len(tags) != 0 {
			for k, v := range tags {
				tag := ssm.Tag{
					Key:   aws.String(k),
					Value: aws.String(v),
				}
				ssmTags = append(ssmTags, &tag)
			}
		}
	}

	return ssmTags
}

func GetSecretsManagerTags() []*secretsmanager.Tag {
	if len(secretsManagerTags) == 0 {
		if len(tags) != 0 {
			for k, v := range tags {
				tag := secretsmanager.Tag{
					Key:   aws.String(k),
					Value: aws.String(v),
				}
				secretsManagerTags = append(secretsManagerTags, &tag)
			}
		}
	}

	return secretsManagerTags
}
