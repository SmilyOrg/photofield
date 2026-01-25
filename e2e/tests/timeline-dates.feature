Feature: Timeline with Historical Dates

  Background:
    Given 80 generated 300 x 200 test photos with dates from "1,100,1000,1500,1800,1900,1950,1969,1970,1971,1990,2000,2020,2030,2040,2100,2500"
    And a running app
    And the user opens the collection

  Scenario: Timeline loads with photos from different decades
    Then the page loads
    And the page shows "100"
    And the page shows "1000"
    And the page shows "1500"
    And the page shows "1800"
    And the page shows "1900"
    And the page shows "1950"
    And the page shows "1969"
    And the page shows "1970"
    And the page shows "1971"
    And the page shows "1990"
    And the page shows "2000"
    And the page shows "2020"
    And the page shows "2030"
    And the page shows "2040"
    And the page shows "2100"
    And the page shows "2500"
