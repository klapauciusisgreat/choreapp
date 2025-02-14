#!/usr/bin/env bash
set -e


# Define the BASE path for the certificates.  This is the part that's
# likely to be constant and is the "single source of truth" for the
# directory structure.
CERT_BASE_PATH="/app/certbot/config/live/${DUCKDNS_SUBDOMAIN}.duckdns.org"

# Export the base path so it's available to sourced scripts.
export CERT_BASE_PATH


# Check if certificates already exist
# Construct the full paths using the CERT_BASE_PATH.
CERT_PATH="${CERT_BASE_PATH}/fullchain.pem"
KEY_PATH="${CERT_BASE_PATH}/privkey.pem"
echo "looking for certificates in $CERT_PATH and $KEY_PATH".

if [ ! -d "$CERT_BASE_PATH" ]; then
  echo "Initial certbot setup"
  # Run certbot to *obtain* the certificates. Use full option names and environment variables.
  /opt/certbot/bin/certbot certonly \
    --authenticator dns-duckdns \
    --dns-duckdns-token=$DUCKDNS_TOKEN \
    -d "$DUCKDNS_SUBDOMAIN.duckdns.org" \
    -m "$EMAIL" \
    --non-interactive --agree-tos \
    --dns-duckdns-propagation-seconds 60 \
    --config-dir /app/certbot/config \
    --work-dir /app/certbot/work \
    --logs-dir /app/certbot/logs


  # Check Certbot's exit status.
  if [ $? -ne 0 ]; then
      echo "Certbot failed during initial setup!"
      exit 1
  fi

  # Final check for certificates (after Certbot finishes).
  if [ ! -f "$CERT_PATH" ]; then
      echo "Certificates were not created even after Certbot ran!"
      exit 1
  fi
else
    echo "Certificates already exist. Skipping initial Certbot run."
fi

# Originally wanted to run cert renewal via cron, but that can't run as non-root user
# unless we use a non-standard cron :(
# Since our scheduling needs are simple, we just run two loops:
# --- DuckDNS Update Loop (runs in the background) ---
(
    while true; do
        /app/scripts/update_duckdns.sh >> /app/duckdns.log 2>&1
	# TODO log errors
        sleep 300  # Sleep for 5 minutes (300 seconds)
    done
) &
DUCKDNS_PID=$!

# --- Certbot Renewal Loop (runs in the background) ---
(
    while true; do
         /opt/certbot/bin/certbot renew --quiet --config-dir /app/certbot/config --work-dir /app/certbot/work --logs-dir /app/certbot/logs
         # Check the exit code; if not 0, there might be an issue
         if [ $? -ne 0 ]; then
           logger -p user.err -t "certbot-renewal" "Certbot renewal failed!"
           # TODO Consider exiting, or at least alerting somehow
         fi
        sleep 43200  # Sleep for 12 hours
    done
) &
CERTBOT_PID=$!

exec "$@"
