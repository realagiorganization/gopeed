Feature: Gopeed CLI downloads
  The Gopeed CLI should handle common download flows reliably.

  Scenario: Download a file from a local HTTP server
    Given a temporary download directory
    And a local HTTP server serving "hello.txt" with content "hello world"
    When I run the gopeed CLI to download the file
    Then the downloaded file "hello.txt" should contain "hello world"

  Scenario: Missing URL shows help
    When I run the gopeed CLI with no arguments
    Then the command should fail with message "missing url parameter"
