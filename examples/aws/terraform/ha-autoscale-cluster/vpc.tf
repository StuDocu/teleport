// VPC for Teleport deployment
resource "aws_vpc" "teleport" {
  cidr_block           = var.vpc_cidr
  enable_dns_support   = true
  enable_dns_hostnames = true
  tags = {
    TeleportCluster = var.cluster_name
  }
}

// Elastic IP for NAT gateways
resource "aws_eip" "nat" {
  count = length(local.azs)
  vpc   = true
  tags = {
    TeleportCluster = var.cluster_name
  }
}

// Internet gateway for NAT gateway
resource "aws_internet_gateway" "teleport" {
  vpc_id = aws_vpc.teleport.id
  tags = {
    TeleportCluster = var.cluster_name
  }
}

// Creates nat gateway per availability zone
resource "aws_nat_gateway" "teleport" {
  count         = length(local.azs)
  allocation_id = element(aws_eip.nat.*.id, count.index)
  subnet_id     = element(aws_subnet.public.*.id, count.index)
  depends_on = [
    aws_subnet.public,
    aws_internet_gateway.teleport,
  ]
  tags = {
    TeleportCluster = var.cluster_name
  }
}

data "aws_vpc" "stagingvpc" {
  default = true
}

resource "aws_vpc_peering_connection" "ConnectionToStagingVPC" {
  peer_owner_id = aws_vpc.teleport.owner_id
  peer_vpc_id   = data.aws_vpc.stagingvpc.id
  vpc_id        = aws_vpc.teleport.id
  tags = {
    Name = "Teleport-Frankfurt"
  }
}

resource "aws_vpc_peering_connection_accepter" "ConnectionAcceptanceToStagingVPC" {
  vpc_peering_connection_id = aws_vpc_peering_connection.ConnectionToStagingVPC.id
  auto_accept               = true
}

locals {
  vpc_id              = aws_vpc.teleport.id
  internet_gateway_id = aws_internet_gateway.teleport.id
  nat_gateways        = aws_nat_gateway.teleport.*.id
}



