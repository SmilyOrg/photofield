Feature: Cleanup

  Scenario: Database files are cleaned up on exit
    Given an empty working directory
    
    When the user runs the app
    Then the app logs "app running"
    And the file "photofield.cache.db" exists
    And the file "photofield.cache.db-shm" exists
    And the file "photofield.cache.db-wal" exists
    And the file "photofield.thumbs.db" exists
    And the file "photofield.thumbs.db-shm" exists
    And the file "photofield.thumbs.db-wal" exists

    When the user stops the app
    Then the app logs "shutdown requested"
    Then the app logs "database closed"
    Then the file "photofield.cache.db" exists
    And the file "photofield.cache.db-shm" does not exist
    And the file "photofield.cache.db-wal" does not exist
    And the file "photofield.thumbs.db" exists
    And the file "photofield.thumbs.db-shm" does not exist
    And the file "photofield.thumbs.db-wal" does not exist
