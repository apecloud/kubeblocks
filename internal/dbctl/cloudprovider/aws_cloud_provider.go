/*
Copyright ApeCloud Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cloudprovider

import (
	"os"
	"path"

	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/internal/dbctl/util"
)

type awsCloudProvider struct {
	region       string
	accessKey    string
	accessSecret string

	plugin *TFPlugin
}

type awsEC2Instance struct {
	PublicIP string
}

const (
	AWSDefaultRegion = "cn-north-1"
)

func (i *awsEC2Instance) GetIP() string {
	return i.PublicIP
}

func NewAWSCloudProvider(accessKey, accessSecret, region string) (CloudProvider, error) {
	if accessKey == "" {
		return nil, errors.New("Invalid access key")
	}
	if accessSecret == "" {
		return nil, errors.New("Invalid access secret")
	}
	if region == "" {
		region = AWSDefaultRegion
	}
	provider := &awsCloudProvider{
		region:       region,
		accessKey:    accessKey,
		accessSecret: accessSecret,
		plugin: NewTFPlugin(
			"terraform-provider-aws_v4.24.0_x5",
			"registry.terraform.io",
			"hashicorp/aws",
			"4.24.0",
		),
	}
	return provider, nil
}

func (p *awsCloudProvider) Name() string {
	return AWS
}

func (p *awsCloudProvider) Apply(destroy bool) error {
	// terraform required auth envs
	_ = os.Setenv("AWS_ACCESS_KEY_ID", p.accessKey)
	_ = os.Setenv("AWS_SECRET_ACCESS_KEY", p.accessSecret)
	publicIP, err := util.GetPublicIP()
	if err != nil {
		return errors.Wrap(err, "Failed to get client public IP")
	}

	// prepare custom ssh keys for aws keypair
	sshDir := path.Join(CLIBaseDir, "ssh")
	publicKeyFile := path.Join(sshDir, "id_rsa.pub")
	if _, err := os.Stat(publicKeyFile); err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrap(err, "Failed to check if ssh public key exists")
		}
		privateKeyFile := path.Join(sshDir, "id_rsa")
		if err := util.MakeSSHKeyPair(publicKeyFile, privateKeyFile); err != nil {
			return errors.Wrap(err, "Failed to init ssh key")
		}
	}
	pubKey, err := os.ReadFile(publicKeyFile)
	if err != nil {
		return errors.Wrap(err, "Failed to read ssh public key")
	}

	// terraform variables
	_ = os.Setenv("TF_VAR_client_public_ip", publicIP)
	_ = os.Setenv("TF_VAR_region", p.region)
	_ = os.Setenv("TF_VAR_ssh_public_key", string(pubKey))

	// install provider plugin
	if err := p.plugin.Install(); err != nil {
		return errors.Wrap(err, "Failed to install provider plugin")
	}

	// terraform apply changes
	tfDir := path.Join(TFBaseDir, AWS)
	if err := tfApply(tfAwsTemplate, tfDir, destroy); err != nil {
		return errors.Wrap(err, "Failed to apply terraform template")
	}

	if destroy {
		if err := os.Remove(providerCfg); err != nil {
			return errors.Wrap(err, "Failed to remove provider config")
		}
	}
	return nil
}

func (p *awsCloudProvider) Instance() (Instance, error) {
	tfDir := path.Join(TFBaseDir, AWS)
	tfState := path.Join(tfDir, "terraform.tfstate")
	instancePublicIP, err := parseInstancePublicIP(tfState)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to query instance public IP")
	}
	return &awsEC2Instance{PublicIP: instancePublicIP}, nil
}

var tfAwsTemplate = `
variable "client_public_ip" {
  type        = string
  description = "Client public IP"
}

variable "region" {
  type        = string
  description = "AWS Region"
}

variable "ssh_public_key" {
  type        = string
  description = "SSH public key"
}

terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 4.16"
    }
  }

  required_version = ">= 1.2.0"
}

provider "aws" {
  region  = "${var.region}"
}

resource "aws_vpc" "main" {
  cidr_block                       = "10.0.0.0/16"
  instance_tenancy                 = "default"
  assign_generated_ipv6_cidr_block = "true"

  tags = {
    Name = "Demo"
  }
}

resource "aws_internet_gateway" "gw" {
  tags = {
    Name = "Demo"
  }
}

resource "aws_internet_gateway_attachment" "example" {
  internet_gateway_id = aws_internet_gateway.gw.id
  vpc_id              = aws_vpc.main.id
}

resource "aws_route_table" "main" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.gw.id
  }

  route {
    ipv6_cidr_block = "::/0"
    gateway_id      = aws_internet_gateway.gw.id
  }

  tags = {
    Name = "Demo"
  }

}

resource "aws_main_route_table_association" "a" {
  vpc_id         = aws_vpc.main.id
  route_table_id = aws_route_table.main.id
}

resource "aws_subnet" "main" {
  vpc_id                          = aws_vpc.main.id
  assign_ipv6_address_on_creation = "true"
  cidr_block                      = "10.0.0.0/24"
  ipv6_cidr_block                 = cidrsubnet(aws_vpc.main.ipv6_cidr_block, 8, 0)

  tags = {
    Name = "Demo"
  }

}

resource "aws_security_group" "main" {
  description = "Demo"
  vpc_id      = aws_vpc.main.id

  egress {
    cidr_blocks      = ["0.0.0.0/0"]
    from_port        = "0"
    ipv6_cidr_blocks = ["::/0"]
    protocol         = "-1"
    self             = "false"
    to_port          = "0"
  }

  ingress {
    cidr_blocks = ["10.0.0.20/32", "${var.client_public_ip}/32"]
    from_port   = "6444"
    protocol    = "tcp"
    self        = "false"
    to_port     = "6444"
  }

  ingress {
    cidr_blocks = ["${var.client_public_ip}/32"]
    from_port   = "22"
    protocol    = "tcp"
    self        = "false"
    to_port     = "22"
  }

  ingress {
    cidr_blocks = ["${var.client_public_ip}/32"]
    from_port   = "3306"
    protocol    = "tcp"
    self        = "false"
    to_port     = "3306"
  }

  ingress {
    cidr_blocks = ["${var.client_public_ip}/32"]
    from_port   = "9100"
    protocol    = "tcp"
    self        = "false"
    to_port     = "9100"
  }

  name = "Demo"

  tags = {
    Name = "Demo"
  }

}

resource "aws_key_pair" "main" {
  public_key = "${var.ssh_public_key}"
}

resource "aws_instance" "main" {
  ami                         = "ami-0e379097467b4afd0"
  associate_public_ip_address = "true"

  subnet_id              = aws_subnet.main.id
  vpc_security_group_ids = [aws_security_group.main.id]

  cpu_core_count       = "1"
  cpu_threads_per_core = "2"

  instance_type      = "t3.medium"
  ipv6_address_count = "1"
  key_name           = aws_key_pair.main.key_name

  private_ip = "10.0.0.10"

  root_block_device {
    delete_on_termination = "true"
    encrypted             = "false"
    volume_size           = "8"
    volume_type           = "gp2"
  }

  tags = {
    Name = "DemoInstance1"
  }
}
`
