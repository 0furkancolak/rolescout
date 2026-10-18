#!/bin/sh
# Runs the labelled evaluation set (30 English and 30 Turkish synthetic job
# descriptions) through a local model and prints accuracy and speed.
#
# Usage: scripts/llm-eval.sh [model] [extra admin flags, e.g. -limit 10]
#
# It talks to LLM_BASE_URL (default http://127.0.0.1:11434/v1, a host Ollama).
# Run it on the machine that has the model; it does not need the database.
set -eu

model="${1:-qwen3:4b-instruct-2507-q4_K_M}"
[ $# -gt 0 ] && shift
cd "$(dirname "$0")/../api"
export LLM_BASE_URL="${LLM_BASE_URL:-http://127.0.0.1:11434/v1}"
export LLM_TIMEOUT="${LLM_TIMEOUT:-300s}"
export DATABASE_URL="${DATABASE_URL:-postgres://unused}"
exec go run ./cmd/admin llm eval -model "$model" "$@"
