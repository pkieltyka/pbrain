# Naming: is `type Item` a Document? Round 1, @lead-dev-2

**Keep `Item`, and do not rename it now or later.** Agree with a one-sentence definition where the type is declared.

An item is the unit an agent cites and reopens: one Markdown file with a header, an id, a kind and a lifecycle. Most items are short and structured, such as a decision with who and when, a learning, or a note. A linked source item may hold only a summary of a document that lives elsewhere. "Document" describes the wrong thing in each of these cases:
- it suggests a long text an agent pages through, whereas `get --offset` and `--outline` exist for the few items that are long;
- it collides with the outside documents that `import` and linked sources bring in, so "a document's item" and "an item's document" would stop being distinguishable.

The alternatives are no better:
- "Note" is already one of the kinds, so it cannot also name all of them;
- "Memory" and "Entry" are vaguer than "item";
- "Record" adds nothing "item" lacks.

"Item" already runs through every surface an agent reads: the spec, the skill, the `item` package, the `item` / `items` words in output and notices, the JSON fields and the id scheme. An agent learns the word once from the skill.

A rename now would touch the spec, the architecture, twelve designs, the skill text and the JSON contract that 6.2 is about to promise only grows. It would be cheaper now than after release, but it buys nothing an agent or a person would notice, so the answer is never rather than later.
