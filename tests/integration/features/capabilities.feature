Feature: Capabilities API
  As a client application
  I want to query system capabilities
  So that I can show users only available features

  Background:
    Given the API is running
    And I am logged in as admin

  Scenario: Get OAuth provider capabilities
    When I get system capabilities
    Then the response status should be 200
    And I should see OAuth providers in capabilities

  Scenario: Get AI provider capabilities
    When I get system capabilities
    Then the response status should be 200
    And I should see AI provider capabilities

  Scenario: Get feature flags
    When I get system capabilities
    Then the response status should be 200
    And I should see feature flags in capabilities

  Scenario: Get operational limits
    When I get system capabilities
    Then the response status should be 200
    And I should see operational limits in capabilities
    And limits should have sync intervals
    And limits should have worker constraints
