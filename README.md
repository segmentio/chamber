# Chamber

Chamber is a tool for managing secrets. Currently it does so by storing
secrets in SSM Parameter Store, an AWS service for storing secrets.

For detailed info about using chamber, please read
[The Right Way To Manage Secrets](https://aws.amazon.com/blogs/mt/the-right-way-to-store-secrets-using-parameter-store/)

## v3.0 Breaking Changes

_Version 3.0 has not yet been released. Changes described here are forward-looking._

* **Use of the SSM Parameter Store's path-based API is now required.** Support
  added in v2.0 to avoid it has been removed. The `CHAMBER_NO_PATHS` environment
  variable no longer has any effect. You must migrate to the new storage format
  using the instructions below.
* **The `--min-throttle-delay` option no longer has any effect.** Support for
  specifying a minimum throttle delay has been removed from the underlying AWS
  SDK with no direct replacement. Instead, set the new `--retry-mode` option to
  "adaptive" to use an experimental model that accounts for throttling errors.

## v2.0 Breaking Changes

Starting with version 2.0, chamber uses parameter store's path based API by default.
Chamber pre-2.0 supported this API using the `CHAMBER_USE_PATHS` environment variable.
The paths based API has performance benefits and is the recommended best practice
by AWS.

As a side effect of this change, if you didn't use path based secrets before 2.0,
you will need to set `CHAMBER_NO_PATHS` to enable the old behavior. This option
is deprecated, and We recommend only using this setting for supporting existing
applications.

To migrate to the new format, you can take advantage of the `export` and `import`
commands. For example, if you wanted to convert secrets for service `foo` to the
new format using chamber 2.0, you can do:

```bash
CHAMBER_NO_PATHS=1 chamber export foo | chamber import foo -
```

### v2.13.0 Breaking Changes

Support for very old versions of Go has been dropped, and chamber will only test
against versions of Go covered by the Go Release Policy, e.g. the two most recent
major versions. This will ensure that we can reliably update dependencies as needed.
Additionally, chamber binaries will be built with the latest stable version of Go
at the time of release.

## Installing

If you have a functional go environment, you can install with:

```bash
go install github.com/segmentio/chamber/v2@latest
```

### Caveat About `chamber version` and `go install`

Note that installing with `go install` will not produce an executable containing
any versioning information. This information is passed at compilation time when
the `Makefile` is used for compilation. Without this information, `chamber version`
outputs the following:

```text
$ chamber version
chamber dev
```

[See the wiki for more installation options like Docker images, Linux packages, and precompiled binaries.](https://github.com/segmentio/chamber/wiki/Installation)

## Authenticating

Using `chamber` requires you to be running in an environment with an
authenticated AWS user which has the appropriate permission to read/write
values to SSM Parameter Store.

This is going to vary based on your organization but chamber needs AWS credentials
to run.

One of the easiest ways to do so is by using [aws-vault](https://github.com/99designs/aws-vault).
To adjust these instructions for your needs, examine the env output of
[Aws-Vault: How It Works](https://github.com/99designs/aws-vault#how-it-works)
and use your organization's secrets tool accordingly with chamber.

### An `aws-vault` Usage Example With Chamber

```bash
aws-vault exec prod -- chamber
```

For this reason, it is recommended that you create an alias in your shell of
choice to save yourself some typing, for example (from my `.zshrc`):

```bash
alias chamberprod='aws-vault exec production -- chamber'
```

## Setting Up KMS

Chamber expects to find a KMS key with alias `parameter_store_key` in the
account that you are writing/reading secrets. You can follow the [AWS KMS
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
the environment variable `CHAMBER_KMS_KEY_ALIAS`. As an example, the following
will use your account's default SSM alias:
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

Secret keys are normalized automatically. The `-` will be `_` and the letters will
be converted to upper case (for example a secret with key `secret_key` and
`secret-key` will become `SECRET_KEY`).

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

Listing secrets with expand parameter should show the key names and values for a
given service, along with other useful metadata including when the secret was
last modified, who modified it, and what the current version is.

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
and executes the given command. Secret keys are converted to upper case (for
example a secret with key `secret_key` will become `SECRET_KEY`).

Secrets from services are loaded in the order specified in the command. For
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

- json (default)
- yaml
- java-properties
- csv
- tsv
- dotenv
- tfvars

File is written to standard output by default but you may specify an output file.

### Caveat About Environment Variables

`chamber` can emit environment variables in both dotenv format and exported shell
environment variables. As `chamber` allows creating key names that are themselves
not valid shell variable names, secrets emitted in this format will have their
keys modified to confirm to POSIX shell environment variable naming rules:

- variable names **must** begin with a letter or an underscore
  - variable names **must not** begin with a number
- variable names **must** only contain letters, numbers, or underscores

#### Notes About Dotenv Format

As there is no formal dotenv spec, `chamber` attempts to
adhere to compliance with [joho/godotenv](https://github.com/joho/godotenv) (which
is itself a port of the Ruby library
[bkeepers/dotenv](https://github.com/bkeepers/dotenv)). The output should be generally
cross-compatible with alternative parsers, but without a formal spec compatibility
is not guaranteed.

Of note:

- all key names will be sanitized according the the POSIX shell rules above, and
cast to uppercase
- all values will be rendered using special characters instead of string literals,
  e.g. newlines replaced with the character `\n`, tabstops replaced with the character
  `\t`, etc.
  - no whitespace trimming will be performed on any values

#### Notes About Exported Environment Variables

Alternatively, `chamber` may be used to set local environment variables directly
with the `chamber env` command. For example,

```shell
source <(chamber env service)`
printf "%s" "$SERVICE_VAR"
```

Note that all secrets printed this way will be prefixed with `export`, so if sourced
inline as in the above example, then any and all secrets will then be available
to any process run after sourcing.

the `env` subcommand supports output formatting in two specific ways:

```text
chamber env -h
Print the secrets from the parameter store in a format to export as environment variables

Usage:
  chamber env <service> [flags]

Flags:
  -p, --preserve-case    preserve variable name case
  -e, --escape-strings   escape special characters in values
```

As `chamber` allows creation of keys with mixed case, `--preserve-case` will ensure
that the original key case is preserved. Note that this will **not** prevent the
key name from being sanitized according to the above POSIX shell rules.
By default, values will be rendered using string literals, e.g. newlines will
be printed as literal newlines, tabstops as literal tabstops. Output may be
emitted using escaped special characters instead (identical to
`chamber export -o dotenv)`) by using the flag `--escape-strings`.

### Importing

```bash
$ chamber import [--normalize-keys] <service> <filepath>
```

`import` provides the ability to import secrets from a json or yaml file (like
the kind you get from `chamber export`).

<!-- prettier-ignore -->
> __Note__
> By default, `import` will **not** normalize key inputs, meaning that keys will
> be written to the secrets backend in the format they exist in the source file.
> In order to normalize keys on import, provide the `--normalize-keys` flag

When normalizing keys, before write, the key will be be first converted to lowercase
to match how `chamber write` handles keys.

Example: `DB_HOST` will be converted to `db_host`.

You can set `filepath` to `-` to instead read input from stdin.

### Deleting

```bash
$ chamber delete [--exact-key] service key
```

`delete` provides the ability to remove a secret from chamber permanently,
including the secret's additional metadata. There is no way to recover a
secret once it has been deleted so care should be taken with this command.

<!-- prettier-ignore -->
> __Note__
> By default, `delete` will normalize any provided keys. To change that behavior,
> provide the `--exact-key` flag to attempt to delete the raw provided key.

Example: Given the following setup,

```bash
$ chamber list service
Key         Version                  LastModified      User
apikey      2                        06-09 17:30:56    daniel-fuentes
APIKEY      1                        06-09 17:30:34    daniel-fuentes
```

Calling

```bash
$ chamber delete --exact-key service APIKEY
```

will delete only `APIKEY` from the service and leave only

```bash
$ chamber list service
Key         Version                  LastModified      User
apikey      2                        06-09 17:30:56    daniel-fuentes
```

### Finding

```bash
$ chamber find key
```

`find` provides the ability to locate which services use the same key names.

```bash
$ chamber find value --by-value
```

Passing `--by-value` or `-v` will search the values of all secrets and return
the services and keys which match.

### Listing Services

```bash
$ chamber list-services [<prefix>]
```

`list-services` lists the available services. You can provide a prefix to limit
the results.

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

If you'd like to use a different region for chamber without changing `AWS_REGION`,
you can use `CHAMBER_AWS_REGION` to override just for chamber.

### Custom SSM Endpoint

If you'd like to use a custom SSM endpoint for chamber, you can use `CHAMBER_AWS_SSM_ENDPOINT`
to override AWS default URL.

## AWS Secrets Manager
Chamber supports AWS Secrets Manager as an optional backend. For example:

```
chamber -b secretsmanager write myservice foo fah
chamber -b secretsmanager write myservice foo2 fah2
```

This will result in one secret being generated with the following JSON value:

```json
{"_chamber_metadata":"{\"foo\":{\"created\":\"2024-06-05T00:32:06.680112Z\",\"created_by\":\"arn:aws:sts::0123456789:assumed-role/yourrole\",\"version\":1},\"foo2\":{\"created\":\"2024-06-05T00:32:39.672526Z\",\"created_by\":\"arn:aws:sts::0123456789:assumed-role/yourrole\",\"version\":1}}",
"foo":"fah",
"foo2":"fah2"}
```

## S3 Backend (Experimental)

By default, chamber store secrets in AWS Parameter Store. We now also provide an
experimental S3 backend for storing secrets in S3 instead.

To configure chamber to use the S3 backend, use `chamber -b s3 --backend-s3-bucket=mybucket`.
Preferably, this bucket should reject uploads that do not set the server side
encryption header ([see this doc for details how](https://aws.amazon.com/blogs/security/how-to-prevent-uploads-of-unencrypted-objects-to-amazon-s3/))

This feature is experimental, and not currently meant for production work.

### S3 Backend using KMS Key Encryption (Experimental)

This backend is similar to the S3 Backend but uses KMS Key Encryption to encrypt
your documents at rest, similar to the SSM Backend which encrypts your secrets
at rest. You can read how S3 Encrypts documents with KMS [here](https://docs.aws.amazon.com/kms/latest/developerguide/services-s3.html).

The highlights of SSE-KMS are:

- You can choose to create and manage encryption keys yourself, or you can choose
  to use your default service key uniquely generated on a customer by service by
  region level.
- The ETag in the response is not the MD5 of the object data.
- The data keys used to encrypt your data are also encrypted and stored alongside
  the data they protect.
- Auditable master keys can be created, rotated, and disabled from the AWS KMS console.
- The security controls in AWS KMS can help you meet encryption-related compliance
  requirements.

Source: [Protecting data using server-side encryption with AWS Key Management Service keys (SSE-KMS)](https://docs.aws.amazon.com/AmazonS3/latest/dev/UsingKMSEncryption.html)

To configure chamber to use the S3 KMS backend, use
`chamber -b s3-kms --backend-s3-bucket=mybucket --kms-key-alias=alias/keyname`.
You must also supply an environment variable of the KMS Key Alias to use
CHAMBER_KMS_KEY_ALIAS, by default "alias/parameter_store_key"
will be used.

Preferably, this bucket should reject uploads that do not set the server side
encryption header ([see this doc for details how](https://aws.amazon.com/blogs/security/how-to-prevent-uploads-of-unencrypted-objects-to-amazon-s3/))

When changing secrets between KMS Keys, you must first delete the Chamber secret
with the existing KMS Key, then write it again with new KMS Key.

If services contain multiple KMS Keys, `chamber list` and `chamber exec` will only
show Chamber secrets encrypted with KMS Keys you have access to.

This feature is experimental, and not currently meant for production work.

## Null Backend (Experimental)

If it's preferred to not use any backend at all, use `chamber -b null`. Doing so
will forward existing ENV variables as if Chamber is not in between.

This feature is experimental, and not currently meant for production work.

## Analytics

`chamber` includes some usage analytics code which Segment uses internally for
tracking usage of internal tools. This analytics code is turned off by default,
and can only be enabled via a linker flag at build time, which we do not set for
public github releases.

## Releasing

To cut a new release, just push a tag named `v<semver>` where `<semver>` is a
valid semver version. This tag will be used by Github Actions to automatically publish
a github release.

---

<div align="center">
THE CHAMBER OF SECRETS HAS BEEN OPENED
</div>
