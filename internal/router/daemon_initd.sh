#!/bin/sh /etc/rc.common

START=95
USE_PROCD=1

start_service() {
    procd_open_instance
    procd_set_param command /usr/bin/opensnitchd-router -config /etc/opensnitchd-router/config.json
    procd_set_param respawn
    procd_close_instance
}
