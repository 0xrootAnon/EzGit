# Safety

- No shell concatenation: commands are built as arg slices.
- Destructive commands detected heuristically; typed confirmation required.
- Credential handling: we rely on system git credential helper; EzGit never stores plaintext credentials.
- Audit logging is opt-in; sensitive outputs are not written unless user enables it.
