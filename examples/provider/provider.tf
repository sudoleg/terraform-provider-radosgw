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

# resource "radosgw_user" "terraformed_user" {
#   user_id      = "terraformed"
#   display_name = "User created by terraform for testing (modified)"
#   keys = [
#     {
#       user = "terraformed"
#     }
#   ]
# }

resource "radosgw_user" "demo_user" {
  user_id      = "demo"
  display_name = "Ceph demo user"
  keys = [
    {
      user = "demo",
    },
    {
      user = "demo",
    },
    # {
    #   user = "demo",
    # }
  ]
}
