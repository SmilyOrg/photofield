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

  Scenario: One Folder
    Given the following files:
      | src                          | dst            |
      | ../docs/assets/logo-wide.jpg | vacation/a.jpg |
    
    When the user runs the app
    Then the app logs "app running"
    
    When the user opens the home page
    Then the page shows "Photos"
    And the page shows "vacation"

  Scenario: One Folder + Add Config
    Given the following files:
      | src                          | dst            |
      | ../docs/assets/logo-wide.jpg | vacation/a.jpg |
    
    When the user runs the app
    And the user opens the home page
    Then the page shows "vacation"

    When the user adds the config "holiday.yaml"
    And waits a second
    And opens the home page
    Then the page shows "holiday"
    And the page does not show "vacation"

  Scenario: Add Folder
    Given an empty working directory
    
    When the user runs the app
    And the user opens the home page
    Then the page shows "Photos"
    And the page shows "No collections"

    When the user adds a folder "photos"
    And the user adds the following files:
      | src                          | dst          |
      | ../docs/assets/logo-wide.jpg | photos/a.jpg |

    When the user waits a second
    And clicks "Retry"
    Then the page does not show "No collections"

  Scenario: Preconfigured Basic
    Given an empty working directory
    And the config "three-collections.yaml"
    
    When the user runs the app
    Then the app logs "app running"
    And the app logs "config path configuration.yaml"
    And the app logs "test123"
    And the app logs "test456"
    And the app logs "test789"
    
    When the user opens the home page
    Then the page shows "Photos"
    
    When the user opens the home page
    Then the page shows "Photos"
    And the page shows "test123"
    And the page shows "test456"
    And the page shows "test789"

  Scenario: Collection One File
    Given the following files:
      | src                          | dst            |
      | ../docs/assets/logo-wide.jpg | vacation/a.jpg |
    
    When the user runs the app
    Then the app logs "app running"
    
    When the user opens "/collections/vacation"
    Then the page shows "vacation"
