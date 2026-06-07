Feature: Pipeline Tasks API

  Scenario: Index Files
    Given an empty working directory
    And the following files:
      | src                          | dst            |
      | ../docs/assets/logo-wide.jpg | vacation/a.jpg |
    And a running app

    When the user posts a task "INDEX_FILES" for collection "vacation"
    Then the task response has status 202
    And the tasks complete
    Then the app logs "index task done index-faces-vacation"

  Scenario: Index Metadata
    Given an empty working directory
    And the following files:
      | src                          | dst            |
      | ../docs/assets/logo-wide.jpg | vacation/a.jpg |
    And a running app

    When the user posts a task "INDEX_FILES" for collection "vacation"
    And the tasks complete

    When the user posts a forced task "INDEX_METADATA" for collection "vacation"
    Then the task response has status 202
    And the tasks complete
    Then the app logs "index metadata extract 1 files"
    And the app logs "index task done index-metadata-vacation"

  Scenario: Index Contents
    Given an empty working directory
    And the following files:
      | src                          | dst            |
      | ../docs/assets/logo-wide.jpg | vacation/a.jpg |
    And a running app

    When the user posts a task "INDEX_FILES" for collection "vacation"
    And the tasks complete

    When the user posts a forced task "INDEX_CONTENTS" for collection "vacation"
    Then the task response has status 202
    And the tasks complete
    Then the app logs "index contents extract 1 files"
    And the app logs "index task done index-contents-vacation"

  Scenario: Index Faces
    Given an empty working directory
    And the following files:
      | src                          | dst            |
      | ../docs/assets/logo-wide.jpg | vacation/a.jpg |
    And a running app

    When the user posts a task "INDEX_FILES" for collection "vacation"
    And the tasks complete

    When the user posts a task "INDEX_FACES" for collection "vacation"
    Then the task response has status 202
    And the tasks complete
    Then the app logs "index task done index-faces-vacation"

  Scenario: Index All
    Given an empty working directory
    And the following files:
      | src                          | dst            |
      | ../docs/assets/logo-wide.jpg | vacation/a.jpg |
    And a running app

    When the user posts a task "INDEX_FILES" for collection "vacation"
    And the tasks complete

    When the user posts a forced task "INDEX_ALL" for collection "vacation"
    Then the task response has status 202
    And the tasks complete
    Then the app logs "index task done index-metadata-vacation"
    And the app logs "index task done index-contents-vacation"
    And the app logs "index task done index-faces-vacation"
