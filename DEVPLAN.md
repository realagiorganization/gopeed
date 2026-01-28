# Development Plan

## Goals
- Maintain a stable Go core and Flutter UI with shared download engine.
- Keep CI pipelines green across desktop, mobile, and web targets.
- Ship signed mobile releases to stores (TestFlight for iOS).

## Step-by-step workflow
1. Install required toolchains (Go + Flutter + platform SDKs listed below).
2. Clone the repo and bootstrap dependencies.
3. Run Go unit tests and BDD scenarios locally.
4. Build and run Flutter UI for your target platform.
5. Validate web build and Docker image if changing web or deployment logic.
6. Cut a release; CI builds artifacts and (when configured) uploads iOS to TestFlight.

## Local commands
- Go unit tests: `go test ./...`
- BDD suite: `go test ./bdd -v`
- Flutter desktop build: `cd ui/flutter && flutter build <platform>`
- Flutter web build: `cd ui/flutter && flutter build web --no-web-resources-cdn`

## External dependencies
- Go toolchain (>= 1.24) and C toolchain for cgo builds.
- Flutter SDK (>= 3.24) and Dart SDK (bundled with Flutter).
- Gomobile (`golang.org/x/mobile/cmd/gomobile`) for iOS/Android bindings.
- Xcode + command line tools (macOS) for iOS builds.
- CocoaPods (macOS) for iOS dependencies.
- Android SDK + Java 17 for Android builds.
- Docker (for container build checks).
- Inno Setup (Windows packaging in CI).
- GitHub Actions runners for CI automation.
- Codecov for coverage reporting.
- App Store Connect API credentials + Apple signing assets for TestFlight deployment.

## Release checklist (high level)
- Ensure tests and BDD suite are green on main.
- Update release notes as needed.
- Publish a GitHub release to trigger the TestFlight workflow.
