<!-- Thanks for contributing to bxw! Please fill this out so review is quick. -->

## What does this PR do?

<!-- One or two sentences. Link the issue it closes: "Closes #123". -->

## How was it tested?

<!-- Commands you ran, browsers you tried, new tests added. bxw's bar is live
     verification, not "should work". -->

## Checklist

- [ ] `go vet ./...` is clean
- [ ] `go test ./... -count=1` passes
- [ ] `GOOS=linux go build ./...` still compiles (the `!windows` stub must keep building)
- [ ] Docs updated if behaviour/flags changed (README CLI table, MCP tool list)
- [ ] **Scope:** this change does not add CAPTCHA solving, bot-detection evasion, stealth/fingerprint spoofing, or anti-automation circumvention. Challenge handling stays detect-and-hand-to-human only.
- [ ] **Privacy:** no profile contents, cookies, or user identity are logged or sent off-machine; any process-matching stays scoped to bxw's own profile dirs.
