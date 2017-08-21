# Chamber

Chamber is a tool for managing secrets.  Currently it does so by storing secrets in SSM Parameter Store, an AWS service for storing secrets.

## Authenticating

Using `chamber` requires you to be running in an environment with an authenticated AWS user which has the appropriate permission to read/write values to SSM Parameter Store.  The easiest way to do so is by using `aws-vault`, like:

```bash
$ aws-vault exec prod -- chamber
```

For this reason, it is recommended that you create an alias in your shell of choice to save yourself some typing, for example (from my `.zshrc`):

```
alias chamberprod='aws-vault exec production -- chamber'
```

## Usage

### Writing Secrets

```bash
$ chamber write <service> <key> <value>
```

This operation will write a secret into the secret store. If a secret with that key already exists, it will increment the version and store a new value.


### Listing Secrets

```bash
$ chamber list service
Key         Version                  LastModified      User
apikey      2                        06-09 17:30:56    daniel-fuentes
other       1                        06-09 17:30:34    daniel-fuentes
```

Listing secrets should show the key names for a given service, along with other useful metadata including when the secret was last modified, who modified it, and what the current version is.


### Historic view

```bash
$ chamber history service key
Event       Version     Date            User
Created     1           06-09 17:30:19  daniel-fuentes
Updated     2           06-09 17:30:56  daniel-fuentes
```
The history command gives a historical view of a given secret. This view is useful for auditing changes, and can point you toward the user who made the change so it's easier to find out why changes were made.

### Exec
```bash
$ chamber exec <service...> -- <your executable>
```

`exec` populates the environment with the secrets from the specified services and executes the given command.  Secret keys are converted to upper case (for example a secret with key `secret_key` will become `SECRET_KEY`).

Secrets from services are loaded in the order specified in the command.  For example, if you do `chamber exec app apptwo -- ...` and both apps have a secret named `api_key`, the `api_key` from `apptwo` will be the one set in your environment.

### Reading
```bash
$ chamber read service key
Key             Value                           Version         LastModified    User
key             secret                          1               06-09 17:30:56  daniel-fuentes
```

`read` provides the ability to print out the value of a single secret, as well as the secret's additional metadata. It does not provide the ability to print out multiple secrets in order to discourage accessing extra secret material that is unneeded. Parameter store automatically versions secrets and passing the `--version/-v` flag to read can print older versions of the secret. Default version (-1) is the latest secret.

## Releasing

To cut a new release, just push a tag named `v<semver>` where `<semver>` is a valid semver version.  This tag will be used by Circle to automatically publish a github release.
