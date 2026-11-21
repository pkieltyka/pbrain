# Should `Item` be `Document`? The group's answer

Drafted by @architect from the round-1 paragraphs of @architect (227227b2), @architect-2 (2e268b70) and @lead-dev (96053414), written independently. @lead-dev-2's paragraph was not in when this was drafted; their line is at the end.

## Recommendation

**Keep `Item`, and say once what it means.** No rename, now or later. All three papers reached this alone.

## Why

- **An item is not always a document.** It is the unit of the brain that can be cited by its id, verified, replaced and fetched: a decision of one sentence, a learning, an observation, a note, or a source. Only a `source` stands for a document, and it either holds a copy of it or only points to it.
- **"Document" already means something else here**, and the difference matters. A document is outside material: what `import` copies, or, for a linked item, what stays at its origin behind someone else's access while the brain holds a pointer and a summary. "Open the item, then read the document at its source" says exactly what happens. The skill leans on the same pair: "from a document, record what is reusable as items". Calling both things "document" would blur the line that sentence draws.
- **The other names are no better.** `Note` is one of the five kinds. `Memory` suggests an agent's private store, and the brain also holds decisions made by people. `Entry` suggests a row in a log. `Record` is as neutral as `Item` and no clearer.
- **A rename costs a lot and buys nothing.** "Item" appears about 350 times in the spec, 270 in the architecture and 600 in the twelve designs, and in output that agents read (`3 replaced items also matched`, the `items` counts of `index` and `tags`). It is cheaper before any code exists than after, but it is still a spec change with every design read again, and it makes neither ids nor navigation clearer.

## What the question does show

The architecture declares `type Item` without saying what an item is. One sentence fixes that, as the doc comment of the type and in architecture 4.1:

> An item is one Markdown file of the brain: one piece of knowledge with its header, cited by its id. It can be a decision, a learning, an observation, a note or a source. A `source` item stands for material from outside, as a copy or as a pointer, with or without a summary; what is reusable in such material can also be recorded as items of the other kinds.

This is a clarification in the architecture and changes nothing in the spec, the output or the designs. @architect adds it with the next architecture edit if the owner agrees.

## Sign-off

- @architect: agree (drafter). Both wording requests below are applied: the "half of the brain" claim is gone, and a `source` is described as standing for outside material, as a copy or as a pointer.
- @architect-2: Agree with keeping `Item` and defining it once. Please drop the unmeasured “half of the brain” claim and describe a `source` as representing outside material, since a linked source may hold only a pointer.
- @lead-dev: agree. One wording refinement: a source item can contain a copied document or point to one. The source kind alone does not imply a document body. The defining sentence can make that distinction without changing the recommendation.
- @lead-dev-2: agree. Keep `Item`, and add the defining sentence as now worded. My reasons are in round1-lead-dev-2.md: "document" would describe the wrong thing for short structured items and would collide with the outside material a source stands for; `Note` is already a kind; and a rename buys an agent nothing.
