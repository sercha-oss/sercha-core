Feature: First Time User Experience
  As an administrator
  I want to set up the system
  So users can start using search

  Scenario: Complete FTUE setup
    Given the API is running
    When I create an admin with email "admin@test.com" and password "password123"
    Then the response status should be 201

    When I login with email "admin@test.com" and password "password123"
    Then the response status should be 200
    And I should receive a token

    When I connect Vespa
    Then the response status should be 200

    # Wait for Vespa container to be fully ready after schema deployment
    When Vespa is fully ready
    Then the system should be healthy

  # Tests for GET /api/v1/setup/status (Public endpoint)
  # Note: Runs after "Complete FTUE setup" so users already exist
  Scenario: Setup status returns valid response after FTUE
    Given the API is running
    When I check setup status
    Then the response status should be 200
    And has_users should be true
    And has_sources should be false
    And vespa_connected should reflect Vespa status

  Scenario: After source creation, has_sources is true
    Given the API is running
    And I am logged in as admin
    And Vespa is fully ready
    When I ensure a localfs installation exists with path "/data/test-docs"
    And I ensure a source exists from container "test-docs"
    And I check setup status
    Then the response status should be 200
    And has_sources should be true

  # Tests for GET /api/v1/settings/ai/providers (Authenticated endpoint)
  Scenario: Authenticated user can get AI providers
    Given the API is running
    And I am logged in as admin
    When I get AI provider metadata
    Then the response status should be 200
    And I should see embedding providers
    And I should see LLM providers
    And providers should include OpenAI
    And providers should include Anthropic
    And providers should include Ollama

  Scenario: Unauthenticated request to ai/providers fails
    Given the API is running
    When I request AI providers without authentication
    Then the response status should be 401

  Scenario: AI providers include required metadata
    Given the API is running
    And I am logged in as admin
    When I get AI provider metadata
    Then the response status should be 200
    And each provider should have an id
    And each provider should have a name
    And each provider should have models
    And each provider should indicate API key requirement
