Feature: Connection Error Message

  Scenario: UI loads, but API is down
    Given an empty working directory
    And no running app
    When the user opens the home page
    Then the page shows "Connection error"

  Scenario: UI loads, API is up intermittently
    Given an empty working directory
    And a running app
    When the user opens the home page
    Then the page shows "Photos"
    Then the page does not show "Connection error"
    And the page shows "No collections"
    When the API goes down
    And the user waits for 2 seconds
    And the user switches away and back to the page
    And the user waits for 5 seconds
    Then the page shows "Connection error"
    When the API comes back up
    Then the page shows "Photos"
    When the user clicks "Retry"
    Then the page does not show "Connecting..."
    And the page does not show "Connection error"

  Scenario: Collection page opens, but API is down
    Given an empty working directory
    And the following files:
      | src                          | dst            |
      | ../docs/assets/logo-wide.jpg | vacation/a.jpg |

    When the user opens "/collections/vacation"
    Then the page shows "Connection error"
