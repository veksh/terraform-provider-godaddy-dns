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
- [x] check github actions
  - [x] PR: fix lint vs docs generation
  - [x] merge: check that release is built ok
- [x] add some unit tests
  - [x] mock client: pass client factory as a param, create mocks in test
  - alt 1: mock server: pass client url for a test server on localhost
  - alt 2: intercept and mock (gock): also testing http client
  - these really are integration tests: testing http client with mock server
- [x] add true API checks to integration tests
- [ ] add more record types
  - [x] implement record type check; disallow SOA etc
  - [x] TXT: just several
  - [ ] NS: just several
  - [ ] MX: + prio
  - mb refuse to mod @ unless GODADDY_ALLOW_ROOTMOD is set?
  - mb SRV: + proto, service, port + prio, weight; mb postpone :)
- [ ] provide documentation
- [ ] publish, test getting it from TF
- [ ] use in static site project
