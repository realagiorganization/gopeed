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

  Scenario: REST API creates and completes a download task
    Given a temporary download directory
    And a local HTTP server serving "rest-file.txt" with content "rest hello"
    And a running REST server
    When I create a REST download task for the served file
    Then the REST task should eventually have status "done"
    And the downloaded file "rest-file.txt" should contain "rest hello"

  Scenario: REST task can be paused and resumed
    Given a temporary download directory
    And a local slow HTTP server serving "slow.bin"
    And a running REST server
    And I create a REST download task for the served file
    When I pause the REST task
    And I continue the REST task
    Then the REST task should eventually have status "done"
