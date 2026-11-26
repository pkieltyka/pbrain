# Mnemosyne's `integrations/hermes`: what host it targets

Retrieved 2026-11-26. This covers identity only; how the memory loop works belongs to the Memory subtopic.

- **Repo:** [mnemosyne-oss/mnemosyne](https://github.com/mnemosyne-oss/mnemosyne), MIT, created 2026-04-05, ~3.7k stars, active (pushed 2026-11-26). Description: "Zero-cloud AI memory that works everywhere. SQLite-backed. One pure-Python dependency."
- **Integration dir:** [integrations/hermes](https://github.com/mnemosyne-oss/mnemosyne/tree/main/integrations/hermes). Its `plugin.yaml`:
  ```yaml
  name: hermes-mnemosyne
  version: 0.7.8
  description: "Native local memory for Hermes - SQLite with vector search, FTS5 hybrid ranking, episodic consolidation, and temporal triples. ..."
  author: Abdias J
  pip_dependencies:
    - mnemosyne-memory[embeddings]>=4.0.0
  ```
- **PyPI:** `mnemosyne-hermes` is the provider package and `mnemosyne-memory` is the core library. Recent tags: provider v0.7.7 and core v4.0.0, both 2026-10-27 ([releases](https://github.com/mnemosyne-oss/mnemosyne/releases)).
- **Host = NousResearch/hermes-agent.** The README title is "Mnemosyne for Hermes Agent". Install with `pipx install mnemosyne-hermes` then `hermes config set memory.provider mnemosyne`, then `hermes memory status`. It registers via the `hermes_agent.memory_providers` entry point. The pinned reference commit `f4ac0bea…` exists in NousResearch/hermes-agent. Nous's marketplace lists it with "requires Hermes >=0.21.4" ([marketplace page](https://hermes-agent.nousresearch.com/docs/plugins/mnemosyne)).
- **Relation to built-in memory.** The README recommends turning off Hermes's built-in MEMORY.md/USER.md: "The built-in MEMORY.md/USER.md system is redundant and just burns tokens". Set `memory.memory_enabled: false` and `user_profile_enabled: false`, but do not disable the memory toolset. This confirms that Hermes has one pluggable memory-provider slot, alongside its own file-based memory.
- **Naming confusion.** An older, separate [AxDSan/mnemosyne](https://github.com/axdsan/mnemosyne) also provides a Hermes provider. Hermes Atlas indexes both. *Inference:* mnemosyne-oss looks like the successor or org home of that project; I did not verify this.
