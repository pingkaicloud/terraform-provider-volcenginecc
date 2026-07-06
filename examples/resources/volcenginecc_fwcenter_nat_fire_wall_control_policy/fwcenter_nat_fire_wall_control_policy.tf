resource "volcenginecc_fwcenter_nat_fire_wall_control_policy" "Example" {
  nat_firewall_id   = "nfw-yepxxxxxxx4vvsxpfs"
  direction         = "in"
  action            = "accept"
  proto             = "TCP"
  source            = "0.0.0.0/0"
  source_type       = "net"
  destination       = "0.0.0.0/0"
  destination_type  = "net"
  dest_port         = "60000"
  dest_port_type    = "port"
  description       = "this is a test"
  prio              = 1
  repeat_type       = "Weekly"
  repeat_days       = [2, 3]
  repeat_start_time = "02:00"
  repeat_end_time   = "04:00"
  start_time        = 1782662400
  end_time          = 1782921540
  status            = true
}