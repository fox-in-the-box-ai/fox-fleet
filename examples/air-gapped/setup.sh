#!/usr/bin/env bash
# setup.sh — prepare and install fox-control images for an air-gapped host.
#
# Usage:
#   ./setup.sh prepare   # Run on a networked machine: pulls and saves images.
#   ./setup.sh install   # Run on the air-gapped host: loads images and generates secrets.

set -euo pipefail

FOX_IMAGE="ghcr.io/fox-in-the-box-ai/cloud:stable"
QDRANT_IMAGE="qdrant/qdrant:v1.14.1"
DATA_ROOT="/var/lib/fox-control"
SECRETS_FILE="${DATA_ROOT}/.secrets"
IMAGES_TAR="fox-images.tar"

prepare() {
    echo "Pulling images..."
    docker pull "${FOX_IMAGE}"
    docker pull "${QDRANT_IMAGE}"

    echo "Saving images to ${IMAGES_TAR}..."
    docker save -o "${IMAGES_TAR}" "${FOX_IMAGE}" "${QDRANT_IMAGE}"

    echo "Pulling Ollama embedding model..."
    ollama pull nomic-embed-text

    # Export the model weights for transfer.
    echo "Saving Ollama model to ollama-model.tar..."
    ollama show nomic-embed-text --modelfile > ollama-model.tar

    echo ""
    echo "Done. Transfer these files to the air-gapped host:"
    echo "  - ${IMAGES_TAR}"
    echo "  - ollama-model.tar"
    echo "  - fox-control.toml"
    echo "  - setup.sh"
}

install() {
    if [ ! -f "${IMAGES_TAR}" ]; then
        echo "Error: ${IMAGES_TAR} not found. Run './setup.sh prepare' on a networked machine first." >&2
        exit 1
    fi

    echo "Loading Docker images from ${IMAGES_TAR}..."
    docker load -i "${IMAGES_TAR}"

    echo "Creating data directory at ${DATA_ROOT}..."
    sudo mkdir -p "${DATA_ROOT}"

    if [ -f "${SECRETS_FILE}" ]; then
        echo "Secrets file already exists at ${SECRETS_FILE} — skipping generation."
    else
        echo "Generating secrets..."
        ADMIN_SECRET="$(head -c 24 /dev/urandom | xxd -p)"
        INSTANCE_PWD="$(head -c 16 /dev/urandom | xxd -p)"

        sudo tee "${SECRETS_FILE}" > /dev/null <<EOF
export FOX_ADMIN_SECRET="${ADMIN_SECRET}"
export FOX_INSTANCE_PASSWORD="${INSTANCE_PWD}"
EOF
        sudo chmod 600 "${SECRETS_FILE}"
        echo "Secrets written to ${SECRETS_FILE} (mode 600)."
    fi

    echo ""
    echo "Next steps:"
    echo "  1. Update the image digest in fox-control.toml:"
    echo "     docker images --digests ghcr.io/fox-in-the-box-ai/fox"
    echo "  2. Start Ollama and load the embedding model."
    echo "  3. Source secrets and start fox-control:"
    echo "     source ${SECRETS_FILE}"
    echo "     fox-control serve --config fox-control.toml"
}

case "${1:-}" in
    prepare) prepare ;;
    install) install ;;
    *)
        echo "Usage: $0 {prepare|install}" >&2
        exit 1
        ;;
esac
