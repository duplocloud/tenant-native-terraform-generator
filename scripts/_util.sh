#!/bin/bash -eu

# OS detection
case "$(uname -s)" in
Darwin)
  export TOOL_OS="darwin"
  ;;
Linux)
  export TOOL_OS="linux"
  ;;
esac

# Utility functions
err() { echo "$0:" "$@" 1>&2 ; }
die() { err "$@" ; exit 1 ; }

# Utility function for a usage message
usage() {
  die "$0:" "usage is as follows" "

  $0 ACCT TENANT [PROJECT]

  Where, ACCT must be 'dev' or 'prod', and PROJECT (if given) must be the name of a Terraform project
"
}

# Utility function to log a command before running it.
logged() {
  echo "$0:" "$@" 1>&2
  "$@"
}

# Utility function to make a duplo API call with curl, and output JSON.
duplo_api() {
    local path="${1:-}"
    [ $# -eq 0 ] || shift

    [ -z "${path:-}" ] && die "internal error: no API path was given"
    [ -z "${duplo_host:-}" ] && die "internal error: duplo_host environment variable must be set"
    [ -z "${duplo_token:-}" ] && die "internal error: duplo_token environment variable must be set"

    curl -Ssf -H 'Content-type: application/json' -H "Authorization: Bearer $duplo_token" "$@" "${duplo_host}/${path}"
}

# Utility function to set up AWS credentials before running a command.
with_aws() {
  # Run the command in the configured way.
  case "${AWS_RUNNER:-duplo-admin}" in
  env)
    [ -z "${profile:-}" ] && die "internal error: no AWS profile selected"
    env AWS_PROFILE="$profile" AWS_SDK_LOAD_CONFIG=1 "$@"
    ;;
  esac
}

# Utility function to run Terraform with AWS credentials.
# Also logs the command.
tf() {
  logged with_aws terraform "$@"
}

# Utility function to run "terraform init" with proper arguments, and clean state.
tf_init() {
  rm -f .terraform/environment .terraform/terraform.tfstate
  tf init "$@"
}
