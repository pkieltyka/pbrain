Run pbrain schema <command> for the flags, operands, JSON output, errors and notices of that command. With no command it lists every callable tool in this build. Add --inputs-only to omit output schemas. Use --json for one compact line.

To invoke a tool on the command line, use pbrain --json <command> --flag=value -- operands. For example:

    pbrain --json search --limit=2 -- "session storage"

Inputs use the flag names exactly, including hyphens. Pass each value of a list with its own flag. For a true boolean pass the flag alone. Put operands after -- in the listed order, each as one argument. Keep a search query together as one argument and quote it in the shell. Follow the tool's rules for options that must or cannot appear together.

Read the output schema for the shape of stdout. Read each JSON error or notice from stderr. An exit of 1 can still carry a check report or a delete preview. A preview requires separate confirmation before deletion.

Use personal only for the person's own request. On the CLI, pass --personal for that call. Keep every personal result off outside services and out of shared files.

A harness can use pbrain schema --format=mcp to obtain MCP tool descriptors, including input and output schemas. The harness supplies the transport.
