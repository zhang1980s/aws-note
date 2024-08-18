provider "aws" {
  region = var.region
}

data "aws_caller_identity" "current" {}

variable "region" {
  description = "The AWS region to deploy resources"
  type        = string
  default     = "ap-southeast-1"
}

output "account_id" {
  description = "AWS Account ID"
  value       = data.aws_caller_identity.current.account_id
}

resource "aws_dynamodb_table" "trade" {
  name         = "trade"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "tid"
  range_key    = "cid"

  attribute {
    name = "tid"
    type = "N"
  }

  attribute {
    name = "cid"
    type = "N"
  }

  global_secondary_index {
    name            = "gsi-cid-tid"
    hash_key        = "cid"
    range_key       = "tid"
    projection_type = "ALL"
  }
}

# resource "aws_dynamodb_table" "dashboard" {
#   name         = "dashboard"
#   billing_mode = "PAY_PER_REQUEST"
#   hash_key     = "kkey"
#   range_key    = "ts"
#
#   attribute {
#     name = "kkey"
#     type = "S"
#   }
#
#   attribute {
#     name = "ts"
#     type = "N"
#   }
# }