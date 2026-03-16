Feature: GET /account-links/{id}
  In order to read an account link
  As an API consumer
  I want GET /account-links/{id} to return correct status and payload

  Scenario: Get an existing account link by id
    Given an account link exists with id "6f2f7a67-8f3c-43a1-9f63-7d9c12cb1f0a", userId "user-1", externalInstitution "Bank One", status "PENDING"
    When I send a GET request to "/account-links/6f2f7a67-8f3c-43a1-9f63-7d9c12cb1f0a"
    Then the response status should be 200
    And response JSON field "id" should equal "6f2f7a67-8f3c-43a1-9f63-7d9c12cb1f0a"

  Scenario: Get an account link with malformed id
    When I send a GET request to "/account-links/not-a-uuid"
    Then the response status should be 400

  Scenario: Method not allowed for GET endpoint
    When I send a POST request to "/account-links/6f2f7a67-8f3c-43a1-9f63-7d9c12cb1f0a"
    Then the response status should be 405
