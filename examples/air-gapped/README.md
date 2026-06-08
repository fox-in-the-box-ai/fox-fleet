# Air-Gapped Fox-Control Setup

Run fox-control on a host with no internet access. All container images and
models are transferred offline.

## Prerequisites

- A networked staging machine with Docker Engine and Ollama installed
- A USB drive or other transfer medium
- Docker Engine on the air-gapped target host

## Workflow

### On the networked machine

1. Run the setup script to pull and save all required images:

   ```bash
   chmod +x setup.sh
   ./setup.sh prepare
   ```

   This creates `fox-images.tar` and `ollama-model.tar` in the current
   directory. Copy them (along with this entire directory) to the
   air-gapped host.

### On the air-gapped host

2. Load images and generate secrets:

   ```bash
   ./setup.sh install
   ```

3. Start Ollama (if not already running) and load the embedding model:

   ```bash
   ollama serve &
   ollama create nomic-embed-text < ollama-model.tar
   ```

4. Update the image digest in `fox-control.toml`:

   ```bash
   docker images --digests ghcr.io/fox-in-the-box-ai/fox
   # Copy the sha256 digest into docker.image in fox-control.toml
   ```

5. Start fox-control:

   ```bash
   source /var/lib/fox-control/.secrets
   fox-control serve --config fox-control.toml
   ```
