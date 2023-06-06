#!/usr/bin/env bash

function swap_off() {
  echo "swap off..."
  swapoff -a
  sed -i /^[^#]*swap*/s/^/\#/g /etc/fstab

  # clean cache
  echo 3 > /proc/sys/vm/drop_caches
  echo
}

function selinux_off() {
  echo "selinux off..."
  setenforce 0
  echo "enforce: $(getenforce)"

  if [ -f /etc/selinux/config ]; then
    sed -ri 's/SELINUX=enforcing/SELINUX=disabled/' /etc/selinux/config
  fi
  echo
}

function firewalld_off() {
  echo "firewall off..."
  systemctl stop firewalld.service 1>/dev/null 2>&1
  systemctl disable firewalld.service 1>/dev/null 2>&1
  systemctl stop ufw 1>/dev/null 2>&1
  systemctl disable ufw 1>/dev/null 2>&1
  echo
}

function replace_in_file() {
    local filename="${1:?filename is required}"
    local match_regex="${2:?match regex is required}"
    local substitute_regex="${3:?substitute regex is required}"
    local posix_regex=${4:-true}

    local result
    local -r del=$'\001'
    if [[ $posix_regex = true ]]; then
        result="$(sed -E "s${del}${match_regex}${del}${substitute_regex}${del}g" "$filename")"
    else
        result="$(sed "s${del}${match_regex}${del}${substitute_regex}${del}g" "$filename")"
    fi
    echo "$result" > "$filename"
}

function sysctl_set_keyvalue() {
    local -r key="${1:?missing key}"
    local -r value="${2:?missing value}"
    local -r conf_file="${3:-"/etc/sysctl.conf"}"
    if grep -qE "^#*\s*${key}" "$conf_file" >/dev/null; then
        replace_in_file "$conf_file" "^#*\s*${key}\s*=.*" "${key} = ${value}"
    else
        echo "${key} = ${value}" >>"$conf_file"
    fi
}

function set_network() {
  echo "set network..."

  sysctl_set_keyvalue "net.ipv4.tcp_tw_recycle" "0"
  sysctl_set_keyvalue "net.ipv4.ip_forward" "1"
  sysctl_set_keyvalue "net.bridge.bridge-nf-call-arptables" "1"
  sysctl_set_keyvalue "net.bridge.bridge-nf-call-ip6tables" "1"
  sysctl_set_keyvalue "net.bridge.bridge-nf-call-iptables" "1"
  sysctl_set_keyvalue "net.ipv4.ip_local_reserved_ports" "30000-32767"

  echo
}

function common_os_setting() {
  swap_off
  selinux_off
  firewalld_off
  set_network
}

function install_hosts() {
  sed -i ':a;$!{N;ba};s@# kubeblocks hosts BEGIN.*# kubeblocks hosts END@@' /etc/hosts
  sed -i '/^$/N;/\n$/N;//D' /etc/hosts

  cat >>/etc/hosts<<EOF
# kubeblocks hosts BEGIN
{{- range .Hosts }}
{{ . }}
{{- end }}
# kubeblocks hosts END
EOF
}

function install_netfilter() {
  modinfo br_netfilter > /dev/null 2>&1
  if [ $? -eq 0 ]; then
     modprobe br_netfilter
     mkdir -p /etc/modules-load.d
     echo 'br_netfilter' > /etc/modules-load.d/kubekey-br_netfilter.conf
  fi

  modinfo overlay > /dev/null 2>&1
  if [ $? -eq 0 ]; then
     modprobe overlay
     echo 'overlay' >> /etc/modules-load.d/kubekey-br_netfilter.conf
  fi
}

function install_ipvs() {
  modprobe ip_vs
  modprobe ip_vs_rr
  modprobe ip_vs_wrr
  modprobe ip_vs_sh

  cat > /etc/modules-load.d/kube_proxy-ipvs.conf << EOF
ip_vs
ip_vs_rr
ip_vs_wrr
ip_vs_sh
EOF

  modprobe nf_conntrack_ipv4 1>/dev/null 2>/dev/null
  if [ $? -eq 0 ]; then
     echo 'nf_conntrack_ipv4' > /etc/modules-load.d/kube_proxy-ipvs.conf
  else
     modprobe nf_conntrack
     echo 'nf_conntrack' > /etc/modules-load.d/kube_proxy-ipvs.conf
  fi
}

install_netfilter
install_ipvs
install_hosts
common_os_setting

sysctl_set_keyvalue "vm.max_map_count" "262144"
sysctl_set_keyvalue "vm.swappiness" "1"
sysctl_set_keyvalue "fs.inotify.max_user_instances" "524288"
sysctl_set_keyvalue "kernel.pid_max" "65535"
sysctl -p

# Make sure the iptables utility doesn't use the nftables backend.
update-alternatives --set iptables /usr/sbin/iptables-legacy >/dev/null 2>&1 || true
update-alternatives --set ip6tables /usr/sbin/ip6tables-legacy >/dev/null 2>&1 || true
update-alternatives --set arptables /usr/sbin/arptables-legacy >/dev/null 2>&1 || true
update-alternatives --set ebtables /usr/sbin/ebtables-legacy >/dev/null 2>&1 || true

ulimit -u 65535
ulimit -n 65535
