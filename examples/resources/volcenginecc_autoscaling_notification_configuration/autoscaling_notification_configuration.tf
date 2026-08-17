resource "volcenginecc_autoscaling_notification_configuration" "example" {
  scaling_group_id  = "scg-xxxxxxxxxxxxxxxxx"
  notification_type = "cloudmonitor"
  event_types = [
    "ScaleOutSuccess",
    "ScaleInSuccess",
    "ScaleOutError",
    "ScaleInError",
    "ScaleOutWarn",
    "ScaleInWarn",
    "ScaleReject",
    "ScaleOutStart",
    "ScaleInStart",
    "ScheduleTaskExpiring",
  ]
}
