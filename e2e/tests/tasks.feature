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
    Then the app logs "indexing files vacation done"

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
    Then the app logs "index metadata extracting 1 files"
    And the app logs "index task index-metadata-vacation completed"

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
    Then the app logs "index contents extracting 1 files"
    And the app logs "index task index-contents-vacation completed"

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
    Then the app logs "index task index-faces-vacation completed"

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
    Then the app logs "index task index-metadata-vacation completed"
    And the app logs "index task index-contents-vacation completed"
    And the app logs "index task index-faces-vacation completed"
