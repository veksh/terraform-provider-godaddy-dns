# GoDaddy DNS Terraform Provider

This is WIP on yet another implementation of GoDaddy DNS provider.
It will manage only DNS resources (no e.g. domain management) and
be flexible regarding existing ones (option to silently import them
or complain if expecting to manage it fully).

# todo
- [X] makefile
- [x] provider
- [x] resource: basic types
- [x] check basic working on real dns
- [ ] further checks
  - [ ] pre-existing records: before first apply; import?
  - [ ] conf drift: external modification
- [ ] add more record types; test txt/mx updates
- [ ] mb some unit tests
- [ ] mb some integration tests; cleanup
- [ ] check github actions
  - [ ] PR: fix lint vs docs generation
  - [ ] merge: check that release is built ok
- [ ] publish, test getting it from TF
