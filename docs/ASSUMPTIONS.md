# Assumptions

- App Store Connect and iOS signing secrets (API key, issuer ID, team ID, p12, and provisioning profile) will be provided in repository secrets for the TestFlight workflow.
- The VHS demo workflow is configured to render the CLI demo gif on demand; the checked-in `docs/ui-test.gif` is derived from available GitHub Pages screenshots until the VHS workflow is run.
- The Go toolchain is available in CI to resolve `github.com/cucumber/godog` and run BDD tests.
