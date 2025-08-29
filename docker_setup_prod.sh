#!/bin/bash

# This script sets up the WriteFreely application using Docker by creating a local directory
# in the current working directory (PWD) to store Docker-related files, initializing the database,
# and performing the initial configuration.

# Installation directory in the same location as the script
INSTALL_DIR="$(pwd)/writefreely"

# Create the installation directory if it doesn't exist
if [ ! -d "$INSTALL_DIR" ]; then
  echo "Creating directory at $INSTALL_DIR..."
  mkdir -p "$INSTALL_DIR"
fi

# Change to the installation directory
cd "$INSTALL_DIR" || exit

# URL for the docker-compose file
COMPOSE_URL="https://raw.githubusercontent.com/writefreely/writefreely/refs/heads/develop/docker-compose.prod.yml"

# Check if docker-compose.yml already exists
if [ ! -f "docker-compose.yml" ]; then
  echo "docker-compose.yml not found. Downloading from $COMPOSE_URL..."

  # Check if curl or wget is available and download the file
  if command -v curl &> /dev/null; then
    curl -o docker-compose.yml "$COMPOSE_URL"
  elif command -v wget &> /dev/null; then
    wget -O docker-compose.yml "$COMPOSE_URL"
  else
    echo "Error: Neither curl nor wget is installed. Please install one of them to proceed."
    exit 1
  fi
else
  echo "docker-compose.yml already exists. Skipping download."
fi

# Prompt the user to edit the docker-compose.yml file
echo "Before continuing, you must edit the docker-compose.yml file to configure the database connection details."
read -p "Press Enter when you have finished editing the file."

# Run the initial command for interactive configuration
echo "Starting WriteFreely configuration..."
docker compose run -it --rm app writefreely config start

echo "Configuration completed. Now generating keys..."

# Generate the required keys
docker compose run -it --rm app writefreely keys generate

# Completion message with update instructions
echo "Setup complete! You can now start WriteFreely with 'docker compose up -d'"
echo "To update WriteFreely in the future, run: 'docker-compose down', 'docker-compose pull', and 'docker-compose up -d'"
