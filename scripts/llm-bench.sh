#!/bin/sh
# Measures a local model server: prompt and generation tokens per second and
# wall time for ~1500-token job prompts. Needs curl and jq.
#
# Usage: scripts/llm-bench.sh [model] [native-base-url]
#   model            default qwen3:4b-instruct-2507-q4_K_M
#   native-base-url  default http://127.0.0.1:11434 (Ollama's own API, which
#                    reports prompt_eval_duration and eval_duration)
set -eu

model="${1:-qwen3:4b-instruct-2507-q4_K_M}"
base="${2:-http://127.0.0.1:11434}"
runs="${LLM_BENCH_RUNS:-10}"

command -v jq >/dev/null 2>&1 || { echo "jq is required" >&2; exit 2; }

# About 1500 tokens of plausible job text (the prompt is repeated filler).
para="We are hiring a senior full stack engineer to build our product with React, TypeScript, Node.js and PostgreSQL. You will design APIs, review code and mentor engineers. The team is remote within Europe. Salary range 90,000 to 120,000 EUR per year. Requirements: five years of experience, strong communication, English fluency. Benefits include equity, a learning budget and 30 days of leave. "
text=""
i=0
while [ "$i" -lt 14 ]; do text="$text$para"; i=$((i + 1)); done

echo "model: $model   runs: $runs   server: $base"
total_pp=0 total_gen=0 n=0
start_all=$(date +%s)
while [ "$n" -lt "$runs" ]; do
  # A different first line each run, so the server cannot reuse its prompt cache.
  unique="Reference $n-$(date +%s)-$RANDOM"
  body=$(jq -n --arg m "$model" --arg t "$unique
$text" '{
    model: $m, stream: false, think: false, options: {temperature: 0, num_predict: 200},
    messages: [{role: "system", content: "Extract technologies, seniority and salary as JSON."},
               {role: "user", content: $t}] }')
  out=$(curl -sS --max-time 300 "$base/api/chat" -d "$body")
  pc=$(printf '%s' "$out" | jq '.prompt_eval_count // 0')
  pd=$(printf '%s' "$out" | jq '.prompt_eval_duration // 1')
  gc=$(printf '%s' "$out" | jq '.eval_count // 0')
  gd=$(printf '%s' "$out" | jq '.eval_duration // 1')
  pp=$(echo "$pc $pd" | awk '{printf "%.1f", $1 / ($2 / 1e9)}')
  gen=$(echo "$gc $gd" | awk '{printf "%.1f", $1 / ($2 / 1e9)}')
  printf 'run %2d: prompt %5s tok  %7s tok/s | generated %4s tok  %6s tok/s\n' "$((n + 1))" "$pc" "$pp" "$gc" "$gen"
  total_pp=$(echo "$total_pp $pp" | awk '{print $1 + $2}')
  total_gen=$(echo "$total_gen $gen" | awk '{print $1 + $2}')
  n=$((n + 1))
done
elapsed=$(( $(date +%s) - start_all ))
echo "mean prompt tok/s: $(echo "$total_pp $runs" | awk '{printf "%.1f", $1 / $2}')"
echo "mean generation tok/s: $(echo "$total_gen $runs" | awk '{printf "%.1f", $1 / $2}')"
echo "wall time: ${elapsed}s for $runs jobs ($(echo "$elapsed $runs" | awk '{printf "%.1f", $1 / $2}') s/job)"
echo "Note memory with: ps -o rss,command -p \$(pgrep -f 'ollama runner' | head -1)"
