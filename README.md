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
- [x] further checks
  - [x] pre-existing records: ok after import
  - [x] conf drift: ok refreshed on plan/apply
- [x] fix integration tests
- [ ] check github actions
  - [x] PR: fix lint vs docs generation
  - [ ] merge: check that release is built ok
- [ ] add some unit tests
  - [ ] mock client: pass client factory as a param, create mocks in test
  - alt 1: mock server: pass client url for a test server on localhost
    - really an integration test: testing http client + server
  - alt 2: intercep and mock (gock): also testing http client
- [ ] add more record types; test txt/mx updates
- [ ] provide documentation
- [ ] publish, test getting it from TF
