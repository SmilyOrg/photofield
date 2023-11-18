Feature: First User Experience

  Scenario: Empty Folder
    Given an empty working directory
    When the user runs the app
    Then the app logs "app running"
    When the user opens the home page
    Then the page shows "Photos"
    Then the page does not show "Connection error"
    And the page shows "No collections"
