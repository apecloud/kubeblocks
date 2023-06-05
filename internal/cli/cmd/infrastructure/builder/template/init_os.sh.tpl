#!/usr/bin/env bash

function swap_off() {
  echo "swap off..."
  swapoff -a
  sed -i /^[^#]*swap*/s/^/\#/g /etc/fstab

  # clean cache
  echo 3 > /proc/sys/vm/drop_caches
}

function selinux_off() {
  echo "selinux off..."
  setenforce 0
  echo "enforce: $(getenforce)"

  if [ -f /etc/selinux/config ]; then
    sed -ri 's/SELINUX=enforcing/SELINUX=disabled/' /etc/selinux/config
  fi
}


function replace_in_file() {
    local filename="${1:?filename is required}"
    local match_regex="${2:?match regex is required}"
    local substitute_regex="${3:?substitute regex is required}"
    local posix_regex=${4:-true}

    local result

    # Use a non-printable character as a 'sed' delimiter to avoid issues
    local -r del=$'\001'
    if [[ $posix_regex = true ]]; then
        result="$(sed -E "s${del}${match_regex}${del}${substitute_regex}${del}g" "$filename")"
    else
        result="$(sed "s${del}${match_regex}${del}${substitute_regex}${del}g" "$filename")"
    fi
    echo "$result" > "$filename"
}

function sysctl_set_property() {
    local -r property="${1:?missing property}"
    local -r value="${2:?missing value}"
    local -r conf_file="/etc/sysctl.conf"
    if grep -qE "^#*\s*${property}" "$conf_file" >/dev/null; then
        replace_in_file "$conf_file" "^#*\s*${property}\s*=.*" "${property} = ${value}" false
    else
        echo "${property} = '${value}'" >>"$conf_file"
    fi
}

function set_network() {
  echo "set network..."

  sysctl_set_property "net.ipv4.tcp_tw_recycle" "0"
  sysctl_set_property "net.ipv4.ip_forward" "1"
  sysctl_set_property "net.bridge.bridge-nf-call-arptables" "1"
  sysctl_set_property "net.bridge.bridge-nf-call-ip6tables" "1"
  sysctl_set_property "net.bridge.bridge-nf-call-iptables" "1"
  sysctl_set_property "net.ipv4.ip_local_reserved_ports" "30000-32767"
}

function firewall_off() {
  echo "firewall off..."
  systemctl stop firewalld 1>/dev/null 2>/dev/null
  systemctl disable firewalld 1>/dev/null 2>/dev/null
  systemctl stop ufw 1>/dev/null 2>/dev/null
  systemctl disable ufw 1>/dev/null 2>/dev/null
}


echo 'vm.max_map_count = 262144' >> /etc/sysctl.conf
echo 'vm.swappiness = 1' >> /etc/sysctl.conf
echo 'fs.inotify.max_user_instances = 524288' >> /etc/sysctl.conf
echo 'kernel.pid_max = 65535' >> /etc/sysctl.conf


#See https://imroc.io/posts/kubernetes/troubleshooting-with-kubernetes-network/
sed -r -i "s@#{0,}?net.ipv4.tcp_tw_recycle ?= ?(0|1)@net.ipv4.tcp_tw_recycle = 0@g" /etc/sysctl.conf

sed -r -i  "s@#{0,}?net.ipv4.ip_forward ?= ?(0|1)@net.ipv4.ip_forward = 1@g" /etc/sysctl.conf
sed -r -i  "s@#{0,}?net.bridge.bridge-nf-call-arptables ?= ?(0|1)@net.bridge.bridge-nf-call-arptables = 1@g" /etc/sysctl.conf
sed -r -i  "s@#{0,}?net.bridge.bridge-nf-call-ip6tables ?= ?(0|1)@net.bridge.bridge-nf-call-ip6tables = 1@g" /etc/sysctl.conf
sed -r -i  "s@#{0,}?net.bridge.bridge-nf-call-iptables ?= ?(0|1)@net.bridge.bridge-nf-call-iptables = 1@g" /etc/sysctl.conf
sed -r -i  "s@#{0,}?net.ipv4.ip_local_reserved_ports ?= ?([0-9]{1,}-{0,1},{0,1}){1,}@net.ipv4.ip_local_reserved_ports = 30000-32767@g" /etc/sysctl.conf
sed -r -i  "s@#{0,}?vm.max_map_count ?= ?([0-9]{1,})@vm.max_map_count = 262144@g" /etc/sysctl.conf
sed -r -i  "s@#{0,}?vm.swappiness ?= ?([0-9]{1,})@vm.swappiness = 1@g" /etc/sysctl.conf
sed -r -i  "s@#{0,}?fs.inotify.max_user_instances ?= ?([0-9]{1,})@fs.inotify.max_user_instances = 524288@g" /etc/sysctl.conf
sed -r -i  "s@#{0,}?kernel.pid_max ?= ?([0-9]{1,})@kernel.pid_max = 65535@g" /etc/sysctl.conf

tmpfile="$$.tmp"
awk ' !x[$0]++{print > "'$tmpfile'"}' /etc/sysctl.conf
mv $tmpfile /etc/sysctl.conf


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
sysctl -p

sed -i ':a;$!{N;ba};s@# kubekey hosts BEGIN.*# kubekey hosts END@@' /etc/hosts
sed -i '/^$/N;/\n$/N;//D' /etc/hosts

cat >>/etc/hosts<<EOF
# kubekey hosts BEGIN
{{- range .Hosts }}
{{ . }}
{{- end }}
# kubekey hosts END
EOF

# Make sure the iptables utility doesn't use the nftables backend.
update-alternatives --set iptables /usr/sbin/iptables-legacy >/dev/null 2>&1 || true
update-alternatives --set ip6tables /usr/sbin/ip6tables-legacy >/dev/null 2>&1 || true
update-alternatives --set arptables /usr/sbin/arptables-legacy >/dev/null 2>&1 || true
update-alternatives --set ebtables /usr/sbin/ebtables-legacy >/dev/null 2>&1 || true

ulimit -u 65535
ulimit -n 65535
