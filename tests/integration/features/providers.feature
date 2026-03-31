Feature: Provider List API
  As a client application
  I want to query available providers
  So that I can show users which providers are configured

  Background:
    Given the API is running
    And I am logged in as admin

  Scenario: List all providers shows configuration status
    When I list all providers
    Then the response status should be 200
    And I should see a list of providers
    And each provider should have a configured status

  Scenario: LocalFS provider is always available
    When I list all providers
    Then the response status should be 200
    And I should see localfs in the provider list
    And localfs should be marked as configured

  Scenario: OAuth providers reflect environment configuration
    When I list all providers
    Then the response status should be 200
    And OAuth provider configuration should match capabilities
