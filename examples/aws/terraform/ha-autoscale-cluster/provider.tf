terraform {
  required_version = "~> 1.0.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 2.0"
    }
    template = {
      source  = "hashicorp/template"
      version = "~> 2.2.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 2.2.1"
    }
  }
  backend "remote" {
    organization = "studocu"

    workspaces {
      name = "teleport-ha"
    }
  }
}

provider "aws" {
  region = var.region
}

variable "aws_max_retries" {
  default = 5
}
