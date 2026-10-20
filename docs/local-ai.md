# Local AI (optional)

RoleScout can use a small language model that runs **on your own computer** to
read job descriptions: it fills in what the rules cannot (salary, remote policy,
technologies outside the keyword list, Turkish postings), writes a one-line
summary and explains in words why a strong match fits your search. It is off by
default, free, works offline, and no job text leaves your machine. The reasoning
and the alternatives are in [ADR 0005](adr/0005-local-llm-enrichment.md).

Everything keeps working without it: rules-based extraction and scoring stay
the baseline, and any value the rules know always wins over the model's.

## What you need
- An Apple Silicon Mac with 16 GB of memory (it also works on other machines
  that can run Ollama or llama.cpp, only slower).
- About 3 GB of free disk for the model (2.5 GB download).
- RoleScout running with `make dev` or Docker Compose.

## 1. Install a model server on the host
Docker on a Mac cannot use the Apple GPU, so the model server runs **on the
host**, not in a container. The worker container reaches it through
`host.docker.internal`.

**Ollama (recommended):**
```bash
brew install ollama
ollama pull qwen3:4b-instruct-2507-q4_K_M     # main model, 2.5 GB
```
Start it with settings that suit a laptop that also runs Docker:
```bash
OLLAMA_KEEP_ALIVE=10m OLLAMA_NUM_PARALLEL=1 ollama serve
```
`OLLAMA_KEEP_ALIVE` unloads the model after it has been idle (frees memory for
Docker); `OLLAMA_NUM_PARALLEL=1` keeps one request at a time. Check the names
against `ollama serve --help` for your version.

To keep the models on another volume, set `OLLAMA_MODELS=/path/to/models` before
`ollama serve` (and when you run `ollama pull`). The release tarball from
https://github.com/ollama/ollama/releases also runs from any directory.

**llama.cpp instead:**
```bash
llama-server -m qwen3-4b-instruct-2507-q4_k_m.gguf --jinja -c 4096 -np 1 --host 127.0.0.1 --port 8081
```
and set `LLM_BASE_URL=http://host.docker.internal:8081/v1`. Any server with an
OpenAI-compatible `/v1/chat/completions` that enforces `response_format`
(`json_schema`) works, including LM Studio.

## 2. Turn it on
The quickest way is **Settings → AI** in the web app (see the hosted-service
section below for the fields). The `.env` variables work too:

In `.env`:
```
LLM_ENABLED=true
LLM_BASE_URL=http://host.docker.internal:11434/v1
LLM_MODEL=qwen3:4b-instruct-2507-q4_K_M
```
then `docker compose up -d worker api` (or `make up`). The other settings are in
[configuration](configuration.md#local-language-model-optional).

Open **Settings → Local AI** (administrator). It shows whether the server is
reachable, whether the model is present, and how many jobs are waiting, read and
failed. **Test connection** sends one tiny request.

From the command line:
```bash
cd api
LLM_BASE_URL=http://127.0.0.1:11434/v1 go run ./cmd/admin llm check
```

## Hosted service with an API key (alternative)
Instead of a model on your computer you can use any OpenAI-compatible service
(OpenAI, OpenRouter, a company gateway …). Nothing needs a restart or an `.env`
edit:

1. Sign in as the administrator and open **Settings → AI**.
2. Pick the **OpenAI** preset or type the base URL (ending in `/v1`) and model,
   paste the **API key**, tick **Turn AI on** and save.
3. Press **Test connection**. A rejected key shows as "can't reach".

The key is stored in the database (table `app_settings`), is never returned by
the API (only its last four characters) and applies within seconds. Values
saved here override `LLM_*` from `.env`; clearing a field brings the `.env`
value back. The service must accept `response_format: json_schema` and
`max_tokens` (OpenAI `gpt-4o-mini` does; the newest reasoning models want
`max_completion_tokens` and are not supported).

**Cost control.** For a hosted service the worker does *not* read the whole
catalog on its own (that could be tens of thousands of paid requests). AI text
is made when you ask for it: on a job with no explanation, open **Why it
matches** and press **Generate AI explanation**. That reads the posting and
writes the reason, at any score; the result is stored, and the per-IP limit is
60 generations an hour. Tick **Read every new job in the background** in
Settings if you want the bulk run anyway (a local model has it on by default).

## What happens
After a research run collects jobs, a separate worker loop reads their
descriptions, a few at a time, **best-matching and saved jobs first**. The
pipeline never waits for it: if the model is off, slow or crashes, runs finish
exactly as without it.

- The model's reply is constrained to a fixed JSON schema, then **validated**:
  technologies and salary numbers must literally appear in the text, a
  seniority must be stated in the title or text, country codes must agree with
  the rules. Anything else is dropped.
- Results are cached by a hash of the description, so the same text is never
  read twice, and an unchanged description is not re-read after a re-sync.
- After three failed requests in a row the worker pauses for ten minutes
  (and uses the fallback model if you configured one).
- Values from the model are marked **AI** in the job pane, with a note that
  they were generated on your computer and may be wrong. They never overwrite
  what the rules found.
- For matches of 70 or more it also writes a one-line explanation in the
  workspace owner's language.

## Speed and memory
Measured here: the 4B model read all 60 labelled postings (30 English, 30 Turkish) with valid JSON every time, perfect seniority and technologies, 92% work arrangement, median 18 s per job on a Mac that was also busy with other work (so expect better on an idle machine). The smaller 1.7B model was **not usable**: only 15% of replies were valid, so no fallback model is configured by default. See the numbers in [ADR 0005](adr/0005-local-llm-enrichment.md#measured-on-this-mac)
and the live run in [operations](operations.md). To measure your own machine:
```bash
scripts/llm-bench.sh qwen3:4b-instruct-2507-q4_K_M    # tokens per second
scripts/llm-eval.sh  qwen3:4b-instruct-2507-q4_K_M    # accuracy on 60 labelled postings
```
A backlog of thousands of jobs takes hours; the queue handles the jobs you read
first before the rest. Lower `LLM_MAX_INPUT_CHARS` (for example `3000`) or use
the 1.7B model if it is too slow or hot.

## Troubleshooting
| Symptom | What to check |
| --- | --- |
| Settings says "Can't reach the model server" | Is `ollama serve` running? On the host `curl http://127.0.0.1:11434/v1/models` should answer. From the worker: `docker compose exec worker wget -qO- http://host.docker.internal:11434/v1/models`. |
| "The model is not on the server" | Run the `ollama pull ...` command the card shows. |
| Jobs stay "waiting" | The enricher is paused (breaker open) or the model is slow; see `docker compose logs worker`. |
| Everything is slow or the laptop is hot | A fanless Air throttles after a few minutes of load. Use the 1.7B model, lower `LLM_MAX_INPUT_CHARS`, or let it run while the machine charges. |
| Docker is starved of memory | Give Docker less, or stop the model server when you do not need it. |

## Privacy
Job descriptions and your profile summary go only to the URL in `LLM_BASE_URL`.
The Settings card warns when that host is not this computer or a private
network. Nothing is logged except counts, model names and error reasons, never
job text.

## Turning it off
Set `LLM_ENABLED=false` (or remove it) and restart the worker and API. Stored
results stay in the database and are used until the job's description changes;
`docker compose exec postgres psql -U rolescout -c "truncate job_enrichments, match_explanations"` removes them.
