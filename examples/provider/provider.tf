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

resource "radosgw_user" "demo_user" {
  user_id      = "demo"
  display_name = "Ceph demo user"
}

resource "radosgw_key" "demo_default_key" {
  user = "demo"
}

resource "radosgw_key" "demo_second_key" {
  user = "demo"
}
