[Network]
Domains={{ config_get("user.workshop.project-id", "") }}.{{ properties.domain }} {{ properties.domain }}

[DHCPv4]
Hostname={{ config_get("user.workshop.name", "") }}-{{ config_get("user.workshop.project-id", "") }}
SendRelease=false

[DHCPv6]
Hostname={{ config_get("user.workshop.name", "") }}-{{ config_get("user.workshop.project-id", "") }}
SendRelease=false
