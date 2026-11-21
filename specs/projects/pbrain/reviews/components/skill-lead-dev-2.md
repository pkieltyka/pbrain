# Review: components/skill.md (@lead-dev-2)

Reviewed: `components/skill.md` at sha256 f2d96858becc77fc40f0662a4e37e0fad7b9041b17451c81d65ca2152984b32f, against functional spec 12 and scenario 23 (sha256 4622a82f), architecture 6.1 (sha256 8d40545f) and @architect's semantic command surface of 2026-11-20 (`search --semantic`, `get --outline`, `pbrain index`).

**Verdict: changes required.**
- The interface, the dispatch with no brain, the embedding and the tests are right.
- The core text is 325 words by `strings.Fields` (I counted the same). It leaves out several points spec 12 lists.
- The four topics are requirements, not text, which leaves product wording to the builder.
- There is room: 75 words remain under the 400-word limit.

## Moderate

1. **The core leaves out points spec 12 requires.** Each needs a few words:
   - **Never read whole folders.** Missing.
   - **What standing means.** Missing: decided (3), then verified (2), then provisional (1). Also that "the weight of each standing is the consumer's rule".
   - **Report what the searches found, never that nothing is known.** The second half is missing. It is the behaviour the retrieval review (`reviews/retrieval/joint.md`) cared most about.
   - **The per-clause counts on an empty result.** "Clause counts can help" doesn't say what they are: the closing line counts each word alone, so the agent should retry once with words that exist.
   - **A related line may be a doubt or supporting evidence.** "Follow relevant related items deliberately" loses that.
   - **Never rewrite the old item.** Missing from "how to change a position".
   - **Write only what would change a future task and can be checked.** Only half-present ("cite evidence another reader can check").
   
   Suggested wording that fits the budget:
   - "Never read whole folders."
   - "Standing: decided, then verified, then provisional; how much each weighs is your project's rule."
   - "An empty result counts each word alone: retry once with words that exist. If still short, report what the searches found, never that nothing is known."
   - "A related line is a pointer: a doubt or supporting evidence."
   - "Never edit the old item."

2. **Hybrid search is named but not usable.** "Start with shared lexical search … Hybrid search is optional" names no flag. The flag is `--semantic`, per @architect's command surface; the amendment hash is still to come.
   - Say when to use it: after a lexical miss, or for a question phrased differently from the items.
   - Say what it adds: results marked `semantic` after the lexical ones.
   - Say what to do with them: treat them as leads and `get` them to check.
   
   "shared lexical search" reads as "don't pass `--personal`", which is not the point; drop "shared". Whether to mention `pbrain index` (a person builds the index once) is your call. If the core leaves it out, the `semantic_not_ready` notice must tell the agent what to do. Re-check against the amendment text once its hash is out.

3. **The topic texts are not written.** The table lists required guidance, so the 300-word texts would be written while coding. They are product output, read by agents, and need review like the core. Ship the four texts in the doc, as for the core, each with its word count. While writing them, `import.md` must also say, from spec 12:
   - "the file is the source's own text, never a reader's summary or paraphrase";
   - "a decision recorded from Slack names the person who decided";
   - the linked-item summary is "written deliberately, never by summarising gated content";
   - put the link alone in `--source`, because a URL inside other text is not treated as a URL source (see my audience review, point 1).

## Mild

4. **Give an example for the time window.** Spec 12 gives `--since 7d`; three words buy a usable example.
5. **Count the words the way the test will.** `strings.Fields` counts a bullet's `-` as a word. That is conservative and fine, but say the test is the authority, so wording reviews use the same count.
6. **Curly quotes in the core.** They are valid UTF-8 but cost more tokens than ASCII quotes in some tokenisers. Consider ASCII quotes.
7. **Pin the JSON forms in `cli.md`.** `{"text": …}` and `{"topics": […]}` are not in the spec. They are fine, but they should be listed where the CLI's JSON shapes are listed, so render and cli agree.

## Checked and fine

- `Core`, `Topics` and `Topic` are pure, with exact embedded bytes, a fixed name-to-file lookup and no path from user input.
- Dispatch happens before discovery and the guard (architecture 6.1).
- Extra operands and unknown topics are `usage`, exit 2, with empty standard output.
- The tests cover scenario 23, the no-brain case with failing doubles, the release binary from an empty directory, and the parser check of the topic examples.
- No roles and no workflow are named.

## Recheck, sha256 b52697294c8d6a055c53e760080e86399016ca5008a4bc8cf1df56d29b6c4c41

**Verdict: approve.**
- **Word counts**, recounted with whitespace splitting: core 358, `import` 219, `claude-artifacts` 101, `google-docs` 79, `browser-use` 96. All are within the 400 and 300 limits.
- **The core now covers** everything spec 12 lists that I found missing:
  - never read whole folders;
  - what standing means, and that its weight is the project's choice;
  - the per-clause counts and the retry;
  - "avoid claiming nothing is known";
  - a related item as a doubt or support;
  - never rewrite the old item;
  - the future-task test;
  - `--semantic`, `--outline` and `pbrain index`.
- **The topic texts are written in the doc.** The import topic includes the URL-alone rule, the deciding person for Slack decisions, and summaries written independently of gated text.
- **The test is authoritative** for the word count.

## Output amendment delta, sha256 763d7e76f94ccbff157ff15e003a6d4e87f10208e14769b7778fb055a3ed6421

**Verdict: approve.** The core's navigation sentence carries spec 12's rule:
- `path:line` is for the agent's own file tools, and `get <id> --offset <n>` is for pbrain at the snippet's body offset;
- `--outline` is for headings, `--context 2` is optional, and `--color` is left to people.

It stays inside the 400-word limit, as counted on the shipped bytes.

One nit, not blocking: a personal item outside the brain has a `personal:` path that no file tool can open (spec 6.2). If a later edit frees a few words, "open it by id when the path is personal:" would save an agent one failed read.
