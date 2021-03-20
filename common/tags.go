package common

import (
	"fmt"
	"io"
  "net/url"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/spf13/cobra"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

var (
	tagsFilePath string
	tags map[string]string
	argTags []string
	tagsString string
	ssmTags []*ssm.Tag
	secretsManagerTags []*secretsmanager.Tag
)

func AddCommandWithTagging(parentCmd *cobra.Command, cmd *cobra.Command) {
	// recommended way to set up tagging - prevents accidental override of pre-run hooks
	SetupTagging(cmd)
	parentCmd.AddCommand(cmd)
}


func SetupTagging(cmd *cobra.Command) {
	// Set up flags required for tagging
  // Wrap existing PreRunE / PreRun hooks with tag parsing

	cmd.PersistentFlags().StringVarP(&tagsFilePath, "tags-file", "T", "",
		`Path to a JSON file, whose key-value pairs will be used as tags.
Tags supplied with --tag override tags with the same keys.`,
	)
	cmd.PersistentFlags().StringSliceVarP(&argTags, "tag", "t", []string{},
		"A single tag in key=value format. Multiple instances possible.",
	)

	// PreRunE enjoys precedence over PreRun, if both are defined
	preRunEfn := cmd.PreRunE
	preRunFn := cmd.PreRun

	if preRunEfn != nil {
		cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
			if err := loadTags(cmd, args); err != nil {
				return err
			}
			if err := preRunEfn(cmd, args); err != nil {
				return err
			}
			return nil
		}
	} else if preRunFn != nil {
		cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
			if err := loadTags(cmd, args); err != nil {
				return err
			}
			preRunFn(cmd, args)
			return nil
		}
	} else {
		cmd.PreRunE = loadTags
	}
}


func loadTags(cmd *cobra.Command, args []string) error {
	if err := loadTagsFile(); err != nil {
		return err
	}
	if err := loadArgumentTags(); err != nil {
		return err
	}
	return nil
}


func loadArgumentTags() error{
	for _, tagString := range argTags {
		tagItem := strings.SplitN(tagString, "=", 2)

		if len(tagItem) != 2 {
			return errors.New(fmt.Sprintf("tag argument %s is not in a key=value format", tagString))
		}

		tags[tagItem[0]] = tagItem[1]
	}
	return nil
}


func loadTagsFile() error {
	if tagsFilePath != "" {
		var tagsFileIn io.Reader
		var err error

		if tagsFilePath == "-" {
			tagsFileIn = os.Stdin
		} else {
			tagsFileIn, err = os.Open(tagsFilePath)
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
					Key: aws.String(k),
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
					Key: aws.String(k),
					Value: aws.String(v),
				}
				secretsManagerTags = append(secretsManagerTags, &tag)
			}
		}
	}

	return secretsManagerTags
}
