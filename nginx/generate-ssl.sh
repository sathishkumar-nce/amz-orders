#!/bin/bash

# Generate self-signed SSL certificates for development/testing
# For production, use Let's Encrypt or proper CA-signed certificates

mkdir -p ssl

echo "Generating self-signed SSL certificate..."

openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout ssl/key.pem \
  -out ssl/cert.pem \
  -subj "/C=US/ST=State/L=City/O=Organization/OU=IT/CN=localhost"

echo "SSL certificates generated in ssl/ directory"
echo "WARNING: These are self-signed certificates for development only!"
echo "For production, use Let's Encrypt or proper CA-signed certificates"
