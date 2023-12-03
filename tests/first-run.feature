Feature: First User Experience

  Scenario: Empty Folder
    Given an empty working directory
    When the user runs the app
    Then the app logs "app running"
    When the user opens the home page
    Then the page shows "Photos"
    Then the page does not show "Connection error"
    And the page shows "No collections"

  Scenario: Empty Folder + Add Folder
    Given an empty working directory
    
    When the user runs the app
    Then the app logs "app running"
    
    When the user opens the home page
    Then the page shows "Photos"
    And the page shows "No collections"

    When the user adds a folder "vacation"
    And waits a second
    And the user clicks "Retry"
    Then the page does not show "No collections"
