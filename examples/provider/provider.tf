terraform {
  required_providers {
    radosgw = {
      source = "spreadshirt/radosgw"
    }
  }
}

provider "radosgw" {
  endpoint = "http://127.0.0.1:9000"
  # set access_key_id and secret_access_key via ACCESS_KEY_ID and SECRET_ACCESS_KEY env variables
}

data "radosgw_buckets" "buckets" {
}

output "demo" {
  value = data.radosgw_buckets.buckets
}
