#!/usr/bin/env bash
set -e

# update-duckdns.sh

# Usage: ./update-duckdns.sh

# Use environment variables for token and subdomain
TOKEN=$DUCKDNS_TOKEN
SUBDOMAIN=$DUCKDNS_SUBDOMAIN

# Use curl to update DuckDNS
curl -s "https://www.duckdns.org/update?domains=$DUCKDNS_SUBDOMAIN.duckdns.org&token=$DUCKDNS_TOKEN&ip="
