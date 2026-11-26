# EKS exige subredes en al menos dos zonas de disponibilidad.
resource "aws_vpc" "inventario" {
  cidr_block           = "10.40.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true
  tags                 = { Name = "voltrix-inventario" }
}

resource "aws_subnet" "inventario" {
  for_each          = { a = "10.40.1.0/24", b = "10.40.2.0/24" }
  vpc_id            = aws_vpc.inventario.id
  cidr_block        = each.value
  availability_zone = "${var.region}${each.key}"
  tags              = { Name = "voltrix-inventario-${each.key}" }
}
