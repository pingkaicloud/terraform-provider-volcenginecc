resource "volcenginecc_efs_mount_point" "primary_efs_mountpoint_case_1" {
  file_system_id      = "vol-1234567890abcdef"
  mount_point_name    = "test-dx-full"
  permission_group_id = "pgroup-default"
  vpc_id              = "vpc-1234567890abcdef"
  subnet_id           = "subnet-1234567890abcdef"

}