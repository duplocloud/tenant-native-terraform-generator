# Terraform code generation for a DuploCloud Tenant

This utility provides a way to export the terraform code that represents the infrastructure deployed in a DuploCloud Tenant. This is often very useful in order to:
- Generate and persist DuploCloud Terraform IaC which can be version controlled in the future.
- Clone a new Tenant based on an already existing Tenant. 

## Prerequisite

1. Install [Go](https://go.dev/doc/install)
2. Install [make](https://www.gnu.org/software/make) tool.
3. Install [Terraform](https://learn.hashicorp.com/tutorials/terraform/install-cli) version greater than or equals to `v0.14.11`
4. Following environment variables to be exported in the shell while running this projects.

```shell
# Required Vars
export customer_name="duplo-masp"
export tenant_name="test"
export cert_arn="arn:aws:acm:us-west-2:128329325849:certificate/1234567890-aaaa-bbbb-ccc-66e7dcd609e1"
export duplo_host="https://msp.duplocloud.net"
export duplo_token="xxx-xxxxx-xxxxxxxx"
export AWS_PROFILE="AWS_PROFILE"
export aws_account_id="1234567890"
```
You can optionally pass following environment variables.

```shell
# Optional Vars
export tenant_project="admin-tenant" # Project name for tenant, Default is admin-tenant.
export tf_version=0.14.11  # Terraform version to be used, Default is 0.14.11.
export validate_tf="false" # Whether to validate generated tf code, Default is true.
export generate_tf_state="false" # Whether to import generated tf resources, Default is false. 
                                 # If true please use 'AWS_PROFILE' environment variable, This is required for s3 backend.
```

## How to run this project to export DuploCloud Provider terraform code?

- Clone this repository.

- Prepare environment variables and export within the shell as mentioned above.

- Run using  following command

  ```shell
  make run
  ```

- **Output** : target folder is created along with customer name and tenant name as mentioned in the environment variables. This folder will contain all terraform projects as mentioned below.
  
    ```
    ├── target                   # Target folder for terraform code
    │   ├── customer-name        # Folder with customer name
    │     ├── tenant-name        # Folder with tenant name
    │          ├── tenant        # Terraform code for tenant and tenant related resources.
    ```

  - **Project : tenant** This projects manages creation of DuploCloud tenant and tenant related resources.
