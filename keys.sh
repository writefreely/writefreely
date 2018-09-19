#!/bin/bash
#
# keys.sh generates keys used for the encryption of certain user data. Because
# user data becomes unrecoverable without these keys, the script and won't 
# overwrite any existing keys unless you explicitly delete them.
#

# Generate cookie encryption and authentication keys
if [[ ! -e "$(pwd)/keys/cookies_enc.aes256" ]]; then
	dd of=$(pwd)/keys/cookies_enc.aes256 if=/dev/urandom bs=32 count=1
else
	echo "cookies key already exists! rm keys/cookies_enc.aes256 if you understand the consquences."
fi
if [[ ! -e "$(pwd)/keys/cookies_auth.aes256" ]]; then
	dd of=$(pwd)/keys/cookies_auth.aes256 if=/dev/urandom bs=32 count=1
else
	echo "cookies authentication key already exists! rm keys/cookies_auth.aes256 if you understand the consquences."
fi

# Generate email encryption key
if [[ ! -e "$(pwd)/keys/email_enc.aes256" ]]; then
	dd of=$(pwd)/keys/email_enc.aes256 if=/dev/urandom bs=32 count=1
else
	echo "email key already exists! rm keys/email_enc.aes256 if you understand the consquences."
fi
