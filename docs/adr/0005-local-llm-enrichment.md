# ADR 0005 - Local LLM Enrichment

Status: Proposed
Date: 2026-10-18

## Context
Rule-based extraction misses salary in free text, remote policy in prose,
technologies outside the keyword list, and Turkish postings, and cannot
explain a match in words. The user wants a small on-device language model to
read job descriptions and other fields. Constraints: free and offline, nothing
leaves the machine, a 16 GB MacBook Air (Apple Silicon, no discrete GPU) with
about 6 to 8 GB left for the model next to Postgres and Docker, "very fast",
good JSON reliability, Turkish and English, a permissive licence. See the
[design addendum](../superpowers/specs/2026-10-17-career-page-research-design.md).

## What was verified and what was not
Verified by reading the sources on 2026-10-18:
- Qwen3-4B-Instruct-2507 model card: Apache-2.0; 4.0B parameters (3.6B
  non-embedding); 262 144-token native context; non-thinking only (no
  `<think>` blocks); IFEval 83.4, MultiIF 69.0, MMLU-ProX 61.6, INCLUDE 60.1,
  BFCL-v3 61.9; recommended sampling temperature 0.7, top-p 0.8, top-k 20.
- Qwen3 announcement: Apache-2.0 for the dense models; 119 languages and
  dialects, Turkish listed under the Turkic family.
- Qwen3-1.7B model card: Apache-2.0; 1.7B parameters (1.4B non-embedding);
  32 768 context; thinking can be switched off.
- Ollama library: `qwen3:4b-instruct-2507-q4_K_M` 2.5 GB, `...-q8_0` 4.3 GB,
  `...-fp16` 8.1 GB, 256K context; `qwen3:1.7b-q4_K_M` 1.4 GB, 40K context.
- Ollama documents JSON-schema structured outputs through the `format` field
  of its native API and states that they also work through the OpenAI-compatible
  `response_format`; its OpenAI page lists `response_format` as supported but
  only describes JSON mode explicitly. Local `/v1` base URL is
  `http://localhost:11434/v1/`; `tool_choice`, `logit_bias`, `n` are not
  supported.
- Gemma Terms of Use: custom agreement with a prohibited-use policy and
  flow-down obligations on redistribution, not an OSI licence.
- SmolLM3-3B: Apache-2.0, six natively supported languages (English, French,
  Spanish, German, Italian, Portuguese); Turkish is not listed.
- llama.cpp Apple Silicon benchmark thread (Llama 7B, Q4_0, llama-bench):
  base M2, 100 GB/s, 10 GPU cores: prompt processing 179.6 tokens/s,
  generation 21.9 tokens/s; M2 8-core GPU 145.9 and 21.7; M3 10-core 186.8
  and 21.3; M1 8-core 118.0 and 14.2; M4 10-core 221.3 and 24.1.
- Docker Desktop for Mac cannot pass the Apple GPU into containers (Ollama's
  Docker guidance and 2026 guides); models in containers fall back to CPU.

Not verified (estimates, labelled as such wherever used):
- Tokens per second for 4B and 1.7B models on an Air. Generation is
  memory-bandwidth bound, so scaling the 7B Q4_0 result (about 3.8 GB of
  weights, about 22 tokens/s on M2/M3) by model size gives roughly 30 to 35
  tokens/s for the 4B Q4_K_M (2.5 GB) and 55 to 70 for the 1.7B (1.4 GB). Prompt
  processing scales with compute rather than bytes, so roughly 200 to 300
  tokens/s for the 4B on a 10-core GPU (fewer on an 8-core Air or M1). Real
  values will be lower because of KV-cache reads, sampling and, on a fanless
  Air, thermal throttling. Task 24 and Task 30 add `scripts/llm-bench.sh` and
  replace this paragraph with measured numbers.
- Head-to-head JSON-extraction accuracy of the candidates. A web search found
  no benchmark comparing them on structured extraction; general benchmarks
  (IFEval, multilingual) are weak proxies. The labelled evaluation set in the
  design is how this gets decided for real.
- That `host.docker.internal` reaches an Ollama bound to 127.0.0.1 on Docker
  Desktop (it is the documented way to reach the host, but not confirmed by
  the sources read). Checked in Task 30.
- MLX server structured-output behaviour.

## Decisions

### Model: Qwen3-4B-Instruct-2507, Q4_K_M
- 2.5 GB on disk, roughly 3.5 to 4 GB resident with a 4 096-token context;
  leaves headroom in the 6 to 8 GB budget.
- Instruct-2507 is non-thinking only: no hidden reasoning tokens, so
  latency is predictable. This matters more than a few benchmark points.
- Apache-2.0, commercial use allowed, no flow-down terms.
- Turkish is a listed language of the Qwen3 family; no other candidate at this
  size has a verified Turkish claim except Gemma, which is licence-restricted.
- Strongest instruction following in its size class per the model card
  (IFEval 83.4), which is the closest published proxy for schema adherence.
  Schema adherence itself is enforced by constrained decoding, not trusted.

### Fallback: Qwen3-1.7B, Q4_K_M
1.4 GB, roughly twice as fast (estimate), same licence and tokenizer family,
so prompts and schemas are shared. Used when the user sets
`LLM_MODEL` to it on weaker machines, or by the worker's degraded mode when
the primary model keeps timing out. Needs thinking disabled (`/no_think` in
the prompt, or the runtime's reasoning-off option); quality for Turkish and
subtle remote-policy phrasing is expected to be noticeably lower and is
measured, not assumed.

### Runtime: Ollama on the host, behind an OpenAI-compatible client
- Simplest for a laptop user: `brew install ollama`, `ollama pull <tag>`, it
  runs Metal, loads on demand, unloads when idle (frees memory for Docker).
- Structured outputs with JSON schema are supported.
- RoleScout talks to `POST {LLM_BASE_URL}/chat/completions` only, so
  `llama-server` (llama.cpp, Metal, `--jinja`, grammar-constrained
  `response_format`), LM Studio or a remote box work by changing the URL.
  llama.cpp is the documented alternative for users who want the last bit of
  speed or an explicit `--ctx-size`/`--parallel`.
- The client has one escape hatch: if Task 24's spike shows Ollama's `/v1`
  does not enforce `json_schema`, it calls the native `/api/chat` with
  `format` behind the same Go interface.

### Integration shape
- Optional worker loop after collection; never in the request path and never
  a pipeline stage. Off by default.
- Constrained decoding plus Go validation plus grounding (extracted
  technologies and salary numbers must appear in the text), then a
  deterministic fallback (keep the rules-based value) on any failure.
- Cache by description hash, model, prompt version and schema version.
- One request at a time by default; bounded batches per tick; circuit breaker.
- Rules-based scoring remains the baseline; AI fields are marked in the UI.

## Alternatives considered

| Option | Why not the default |
|---|---|
| Qwen2.5-3B-Instruct | Apache-2.0 only for some sizes (the 3B is under the Qwen research licence, to verify before any use); older than Qwen3 |
| Llama 3.2 3B | Custom community licence with an acceptable-use policy and a naming/attribution clause; Turkish is not among its officially supported languages (8 languages) |
| Gemma 3 4B / 1B | Strong, but the Gemma Terms of Use are custom with flow-down obligations on redistribution; avoided to keep the project's permissive posture |
| Phi-4-mini (3.8B) | MIT and good at reasoning, but primarily English-centric (Turkish support not verified); no verified advantage for extraction |
| SmolLM3-3B | Apache-2.0 and fast, but Turkish is not a supported language (verified) and reasoning mode is on by default |
| Qwen3-8B | 5.2 GB download; better quality but about half the speed of the 4B and too heavy next to Docker on 16 GB |
| Qwen3-4B (hybrid, original) | Same weights family but thinking is on by default and must be disabled; the 2507 instruct build removes that failure mode |
| MLX / mlx-lm server | Often the fastest on Apple Silicon, but a Python runtime to install, less certain constrained-decoding support (not verified), smaller user base; revisit if measurements show a large gap |
| Hosted LLM APIs | Not free, need keys, send job and profile text away; excluded by the design |
| Rules only | Stays the baseline; cannot summarise or explain |

Licence statements for Llama 3.2, Phi-4-mini and Qwen2.5-3B above come from
general knowledge and were not re-verified in this pass; confirm before ever
switching to them.

## Consequences
- RoleScout gains an optional dependency on a host process the project does
  not ship. `make dev` works without it; every feature has a rules-based
  fallback; CI uses a fake server only.
- Expect hours, not minutes, to enrich a large backlog on an Air (see the
  estimates). The queue prioritises matched and saved jobs, so the jobs the
  user reads first are enriched first.
- Apple Silicon only gets Metal when the server runs on the host, so the
  worker container reaches it through `host.docker.internal`; the model
  endpoint must not be exposed to the network.
- Extracted data is probabilistic. It is labelled, grounded and never
  overrides a rule-derived value.
- Model upgrades are config changes plus a re-run of the evaluation set; the
  model name in each row makes mixed generations visible.
- Revisit this ADR when measured numbers are in, when a Qwen3.5-class small
  model or a better Turkish-capable model appears, or if Ollama's structured
  output behaviour changes.

## Sources
- https://huggingface.co/Qwen/Qwen3-4B-Instruct-2507
- https://huggingface.co/Qwen/Qwen3-1.7B
- https://qwenlm.github.io/blog/qwen3/
- https://ollama.com/library/qwen3/tags
- https://docs.ollama.com/capabilities/structured-outputs
- https://docs.ollama.com/openai
- https://github.com/ggml-org/llama.cpp/discussions/4167
- https://ai.google.dev/gemma/terms
- https://huggingface.co/HuggingFaceTB/SmolLM3-3B
