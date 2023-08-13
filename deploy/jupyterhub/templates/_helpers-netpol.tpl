{{- /*
  This named template renders egress rules for NetworkPolicy resources based on
  common configuration.

  It is rendering based on the `egressAllowRules` and `egress` keys of the
  passed networkPolicy config object. Each flag set to true under
  `egressAllowRules` is rendered to a egress rule that next to any custom user
  defined rules from the `egress` config.

  This named template needs to render based on a specific networkPolicy
  resource, but also needs access to the root context. Due to that, it
  accepts a list as its scope, where the first element is supposed to be the
  root context and the second element is supposed to be the networkPolicy
  configuration object.

  As an example, this is how you would render this named template from a
  NetworkPolicy resource under its egress:

    egress:
      # other rules here...

      {{- with (include "jupyterhub.networkPolicy.renderEgressRules" (list . .Values.hub.networkPolicy)) }}
      {{- . | nindent 4 }}
      {{- end }}

  Note that the reference to privateIPs and nonPrivateIPs relate to
  https://en.wikipedia.org/wiki/Private_network#Private_IPv4_addresses.
*/}}

{{- define "jupyterhub.networkPolicy.renderEgressRules" -}}
{{- $root := index . 0 }}
{{- $netpol := index . 1 }}
{{- if or (or $netpol.egressAllowRules.dnsPortsCloudMetadataServer $netpol.egressAllowRules.dnsPortsKubeSystemNamespace) $netpol.egressAllowRules.dnsPortsPrivateIPs }}
- ports:
    - port: 53
      protocol: UDP
    - port: 53
      protocol: TCP
  to:
  {{- if $netpol.egressAllowRules.dnsPortsCloudMetadataServer }}
    # Allow outbound connections to DNS ports on the cloud metadata server
    - ipBlock:
        cidr: {{ $root.Values.singleuser.cloudMetadata.ip }}/32
  {{- end }}
  {{- if $netpol.egressAllowRules.dnsPortsKubeSystemNamespace }}
    # Allow outbound connections to DNS ports on pods in the kube-system
    # namespace
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: kube-system
  {{- end }}
  {{- if $netpol.egressAllowRules.dnsPortsPrivateIPs }}
    # Allow outbound connections to DNS ports on destinations in the private IP
    # ranges
    - ipBlock:
        cidr: 10.0.0.0/8
    - ipBlock:
        cidr: 172.16.0.0/12
    - ipBlock:
        cidr: 192.168.0.0/16
  {{- end }}
{{- end }}

{{- if $netpol.egressAllowRules.nonPrivateIPs }}
# Allow outbound connections to non-private IP ranges
- to:
    - ipBlock:
        cidr: 0.0.0.0/0
        except:
          # As part of this rule:
          # - don't allow outbound connections to private IPs
          - 10.0.0.0/8
          - 172.16.0.0/12
          - 192.168.0.0/16
          # - don't allow outbound connections to the cloud metadata server
          - {{ $root.Values.singleuser.cloudMetadata.ip }}/32
{{- end }}

{{- if $netpol.egressAllowRules.privateIPs }}
# Allow outbound connections to private IP ranges
- to:
    - ipBlock:
        cidr: 10.0.0.0/8
    - ipBlock:
        cidr: 172.16.0.0/12
    - ipBlock:
        cidr: 192.168.0.0/16
{{- end }}

{{- if $netpol.egressAllowRules.cloudMetadataServer }}
# Allow outbound connections to the cloud metadata server
- to:
    - ipBlock:
        cidr: {{ $root.Values.singleuser.cloudMetadata.ip }}/32
{{- end }}

{{- with $netpol.egress }}
# Allow outbound connections based on user specified rules
{{ . | toYaml }}
{{- end }}
{{- end }}
