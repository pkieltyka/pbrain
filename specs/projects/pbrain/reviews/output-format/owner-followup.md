# Owner follow-up (2026-11-21, verbatim)

Quoting the operator's initial view:
> - Line numbers are probably worth adding. Agents often open the file directly with their own tools, and path:line is how they work.
> - A per-note match count is cheap.
> - Context lines are the hard one. The default search budget is 2,000 characters for 5 results, so a few lines either side of each snippet would crowd out results. Likely as an opt-in flag, not the default.
> - A friendlier terminal mode (colour when output goes to a terminal) fits, as long as the plain form stays the stable contract for agents.

> agree with you.. and yes we need to line numbers / position or offset returned so agent can traverse these docs easily.. would be nice i think to return the total number of lines or characters in the doc too, just as a thing to return in the results which is easy for prbrain to compute in go, and useful to to agent to know its total length, etc.. make sure you account for front-matter too.. so the body and frontmatter are technically different, so we should specify the offset or line which the body starts on after the frontmatter..
>
> ask the team what they think is best to return so its clear and easy for the agent to work with once it gets search results
