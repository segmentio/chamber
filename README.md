# Chamber

Chamber is a tool for managing secrets.  Currently it does so by storing
secrets in SSM Parameter Store, an AWS service for storing secrets.

For detailed info about using chamber, read [The Right Way To Manage Secrets](https://aws.amazon.com/blogs/mt/the-right-way-to-store-secrets-using-parameter-store/)

## 2.0 Breaking Changes

Starting with version 2.0, chamber uses parameter store's path based API by default.  Chamber pre-2.0 supported this API using the `CHAMBER_USE_PATHS` environment variable.  The paths based API has performance benefits and is the recommended best practice by AWS.

As a side effect of this change, if you didn't use path based secrets before 2.0, you will need to set `CHAMBER_NO_PATHS` to enable the old behavior.  This option is deprecated, and We recommend only using this setting for supporting existing applications.

To migrate to the new format, you can take advantage of the `export` and `import` commands.  For example, if you wanted to convert secrets for service `foo` to the new format using chamber 2.0, you can do:

```bash
$ CHAMBER_NO_PATHS=1 chamber export foo | chamber import foo -
```

## Installing

If you have a functional go environment, you can install with:

```bash
$ go get github.com/segmentio/chamber
```

[See the wiki for more installation options like Docker images, Linux packages, and precompiled binaries.](https://github.com/segmentio/chamber/wiki/Installation)

## Authenticating

Using `chamber` requires you to be running in an environment with an
authenticated AWS user which has the appropriate permission to read/write
values to SSM Parameter Store.  The easiest way to do so is by using
`aws-vault`, like:

```bash
$ aws-vault exec prod -- chamber
```

For this reason, it is recommended that you create an alias in your shell of
choice to save yourself some typing, for example (from my `.zshrc`):

```
alias chamberprod='aws-vault exec production -- chamber'
```

## Setting up KMS

Chamber expects to find a KMS key with alias `parameter_store_key` in the
account that you are writing/reading secrets.  You can follow the [AWS KMS
documentation](http://docs.aws.amazon.com/kms/latest/developerguide/create-keys.html)
to create your key, and [follow this guide to set up your
alias](http://docs.aws.amazon.com/kms/latest/developerguide/programming-aliases.html).

If you are a [Terraform](https://www.terraform.io/) user, you can create your
key with the following:

```HCL
resource "aws_kms_key" "parameter_store" {
  description             = "Parameter store kms master key"
  deletion_window_in_days = 10
  enable_key_rotation     = true
}

resource "aws_kms_alias" "parameter_store_alias" {
  name          = "alias/parameter_store_key"
  target_key_id = "${aws_kms_key.parameter_store.id}"
}
```

If you'd like to use an alternate KMS key to encrypt your secrets, you can set
the environment variable `CHAMBER_KMS_KEY_ALIAS`. As an example, the following will use your account's default SSM alias:
`CHAMBER_KMS_KEY_ALIAS=aws/ssm`

## Usage

### Writing Secrets

```bash
$ chamber write <service> <key> <value|->
```

This operation will write a secret into the secret store. If a secret with that
key already exists, it will increment the version and store a new value.

If `-` is provided as the value argument, the value will be read from standard
input.


### Listing Secrets

```bash
$ chamber list service
Key         Version                  LastModified      User
apikey      2                        06-09 17:30:56    daniel-fuentes
other       1                        06-09 17:30:34    daniel-fuentes
```

Listing secrets should show the key names for a given service, along with other
useful metadata including when the secret was last modified, who modified it,
and what the current version is.

```bash
$ chamber list -e service
Key         Version                  LastModified      User             Value
apikey      2                        06-09 17:30:56    daniel-fuentes   apikeyvalue
other       1                        06-09 17:30:34    daniel-fuentes   othervalue
```

Listing secrets with expand parameter should show the key names and values for a given service, along with other useful metadata including when the secret was last modified, who modified it,
and what the current version is.

### Historic view

```bash
$ chamber history service key
Event       Version     Date            User
Created     1           06-09 17:30:19  daniel-fuentes
Updated     2           06-09 17:30:56  daniel-fuentes
```
The `history` command gives a historical view of a given secret. This view is
useful for auditing changes, and can point you toward the user who made the
change so it's easier to find out why changes were made.

### Exec
```bash
$ chamber exec <service...> -- <your executable>
```

`exec` populates the environment with the secrets from the specified services
and executes the given command.  Secret keys are converted to upper case (for
example a secret with key `secret_key` will become `SECRET_KEY`).

Secrets from services are loaded in the order specified in the command.  For
example, if you do `chamber exec app apptwo -- ...` and both apps have a secret
named `api_key`, the `api_key` from `apptwo` will be the one set in your
environment.

### Reading
```bash
$ chamber read service key
Key             Value                           Version         LastModified    User
key             secret                          1               06-09 17:30:56  daniel-fuentes
```

`read` provides the ability to print out the value of a single secret, as well
as the secret's additional metadata. It does not provide the ability to print
out multiple secrets in order to discourage accessing extra secret material
that is unneeded. Parameter store automatically versions secrets and passing
the `--version/-v` flag to read can print older versions of the secret. Default
version (-1) is the latest secret.

### Exporting
```bash
$ chamber export [--format <format>] [--output-file <file>]  <service...>
{"key":"secret"}
```

`export` provides ability to export secrets in various file formats. The following
file formats are supported:

* json (default)
* java-properties
* csv
* tsv
* dotenv

File is written to standard output by default but you may specify an output
file.

### Importing
```bash
$ chamber import <service> <filepath>
```

`import` provides the ability to import secrets from a json file (like the kind
you get from `chamber export`).

You can set `filepath` to `-` to instead read input from stdin.

### Deleting
```bash
$ chamber delete service key
```

`delete` provides the ability to remove a secret from chamber permanently,
including the secret's additional metadata. There is no way to recover a
secret once it has been deleted so care should be taken with this command.

### AWS Region

Chamber uses [AWS SDK for Go](https://github.com/aws/aws-sdk-go). To use a
region other than what is specified in `$HOME/.aws/config`, set the environment
variable "AWS_REGION".

```bash
$ AWS_REGION=us-west-2 chamber list service
Key         Version                  LastModified      User
apikey      3                        07-10 09:30:41    daniel-fuentes
other       1                        07-10 09:30:35    daniel-fuentes
```

Chamber does not currently read the value of "AWS_DEFAULT_REGION". See
[https://github.com/aws/aws-sdk-go#configuring-aws-region](https://github.com/aws/aws-sdk-go#configuring-aws-region)
for more details.

If you'd like to use a different region for chamber without changing `AWS_REGION`, you can use `CHAMBER_AWS_REGION` to override just for chamber.

### Custom SSM Endpoint

If you'd like to use a custom SSM endpoint for chamber, you can use `CHAMBER_AWS_SSM_ENDPOINT` to override AWS default URL.

## S3 Backend (experimental)

By default, chamber store secrets in AWS Parameter Store.  We now also provide an experimental S3 backend for storing secrets in S3 instead.

To configure chamber to use the S3 backend, use `chamber -b s3 --backend-s3-bucket=mybucket`.  Preferably, this bucket should reject uploads that do not set the server side encryption header ([see this doc for details how](https://aws.amazon.com/blogs/security/how-to-prevent-uploads-of-unencrypted-objects-to-amazon-s3/))

This feature is experimental, and not currently meant for production work.

## Null Backend (experimental)

If it's preferred to not use any backend at all, use `chamber -b null`. Doing so will forward existing ENV variables as if Chamber is not in between.

This feature is experimental, and not currently meant for production work.


## Analytics

`chamber` includes some usage analytics code which Segment uses internally for tracking usage of internal tools.  This analytics code is turned off by default, and can only be enabled via a linker flag at build time, which we do not set for public github releases.

## Releasing

To cut a new release, just push a tag named `v<semver>` where `<semver>` is a
valid semver version.  This tag will be used by Circle to automatically publish
a github release.
