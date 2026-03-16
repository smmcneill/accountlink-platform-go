package bdd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

type accountLink struct {
	ID                  string `json:"id"`
	UserID              string `json:"userId"`
	ExternalInstitution string `json:"externalInstitution"`
	Status              string `json:"status"`
}

type endpointWorld struct {
	baseURL string
	aliases map[string]string

	lastResponse *http.Response
	lastBody     []byte
}

func newEndpointWorld() *endpointWorld {
	baseURL := strings.TrimRight(os.Getenv("BDD_BASE_URL"), "/")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	return &endpointWorld{
		baseURL: baseURL,
		aliases: map[string]string{},
	}
}

func (w *endpointWorld) reset() {
	if w.lastResponse != nil && w.lastResponse.Body != nil {
		_ = w.lastResponse.Body.Close()
	}
	w.lastResponse = nil
	w.lastBody = nil
}

func (w *endpointWorld) resolvePath(path string) string {
	if strings.HasPrefix(path, "/account-links/") {
		id := strings.TrimPrefix(path, "/account-links/")
		if mapped, ok := w.aliases[id]; ok {
			return "/account-links/" + mapped
		}
	}
	return path
}

func (w *endpointWorld) addAccountLink(id, userID, institution, status string) error {
	body, err := json.Marshal(map[string]string{
		"userId":              userID,
		"externalInstitution": institution,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, w.baseURL+"/account-links", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	respBody, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to seed account link, status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var created accountLink
	if err := json.Unmarshal(respBody, &created); err != nil {
		return fmt.Errorf("failed to parse seed response: %w", err)
	}
	if created.ID == "" {
		return fmt.Errorf("seed response missing id")
	}
	if status != "" && created.Status != status {
		return fmt.Errorf("seed account link status mismatch: expected=%s actual=%s", status, created.Status)
	}

	w.aliases[id] = created.ID
	fmt.Printf("seeded account link: requested_id=%s actual_id=%s\n", id, created.ID)
	return nil
}

func (w *endpointWorld) sendRequest(method, path string) error {
	w.reset()

	target := w.baseURL + w.resolvePath(path)
	fmt.Printf("request call: %s %s\n", method, target)

	req, err := http.NewRequest(method, target, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	w.lastResponse = resp
	w.lastBody, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return nil
}

func (w *endpointWorld) sendPostRequest(path, userID, externalInstitution string) error {
	return w.sendRequestWithBody(http.MethodPost, path, map[string]string{
		"userId":              userID,
		"externalInstitution": externalInstitution,
	})
}

func (w *endpointWorld) sendMalformedPost(path string) error {
	w.reset()

	target := w.baseURL + w.resolvePath(path)
	fmt.Printf("request call: %s %s (malformed JSON)\n", http.MethodPost, target)

	req, err := http.NewRequest(http.MethodPost, target, strings.NewReader(`{"userId":"user-1",`))
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	w.lastResponse = resp
	w.lastBody, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return nil
}

func (w *endpointWorld) sendRequestWithBody(method, path string, payload map[string]string) error {
	w.reset()

	target := w.baseURL + w.resolvePath(path)
	fmt.Printf("request call: %s %s\n", method, target)

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(method, target, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	w.lastResponse = resp
	w.lastBody, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return nil
}

func (w *endpointWorld) responseStatusShouldBe(expected int) error {
	if w.lastResponse == nil {
		return fmt.Errorf("no response captured")
	}
	if got := w.lastResponse.StatusCode; got != expected {
		return fmt.Errorf("expected status %d, got %d", expected, got)
	}
	return nil
}

func (w *endpointWorld) responseHeaderShouldMatchLocationTemplate(header, template string) error {
	if w.lastResponse == nil {
		return fmt.Errorf("no response captured")
	}
	got := w.lastResponse.Header.Get(header)
	if got == "" {
		return fmt.Errorf("expected header %q to be present", header)
	}
	if template != "/account-links/{uuid}" {
		return fmt.Errorf("unexpected template %q", template)
	}
	if !strings.HasPrefix(got, "/account-links/") {
		return fmt.Errorf("expected Location to start with /account-links/, got %q", got)
	}

	candidate := strings.TrimPrefix(got, "/account-links/")
	if _, err := uuid.Parse(candidate); err != nil {
		return fmt.Errorf("expected Location to contain uuid, got %q", got)
	}

	return nil
}

func (w *endpointWorld) responseJSONFieldShouldEqual(field, expected string) error {
	if w.lastResponse == nil {
		return fmt.Errorf("no response captured")
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.lastBody, &body); err != nil {
		return fmt.Errorf("failed to parse json body: %w", err)
	}
	got, ok := body[field]
	if !ok {
		return fmt.Errorf("field %q not found in response", field)
	}

	if field == "id" {
		if mapped, ok := w.aliases[expected]; ok {
			expected = mapped
		}
	}

	return assertJSONFieldString(field, got, expected)
}

func assertJSONFieldString(field string, got interface{}, expected string) error {
	switch v := got.(type) {
	case string:
		if v != expected {
			return fmt.Errorf("expected %q=%q, got %q", field, expected, v)
		}
	default:
		return fmt.Errorf("field %q is not a string", field)
	}

	return nil
}

func InitializeScenario(ctx *godog.ScenarioContext) {
	state := newEndpointWorld()

	ctx.Step(`^an account link exists with id "([^"]+)", userId "([^"]+)", externalInstitution "([^"]+)", status "([^"]+)"$`, state.addAccountLink)
	ctx.Step(`^I send a GET request to "([^"]+)"$`, func(path string) error { return state.sendRequest(http.MethodGet, path) })
	ctx.Step(`^I send a POST request to "([^"]+)" with userId "([^"]+)" and externalInstitution "([^"]+)"$`, state.sendPostRequest)
	ctx.Step(`^I send a POST request to "([^"]+)" with malformed JSON$`, state.sendMalformedPost)
	ctx.Step(`^I send a POST request to "([^"]+)"$`, func(path string) error { return state.sendRequest(http.MethodPost, path) })
	ctx.Step(`^the response status should be (\d+)$`, func(status string) error {
		expected, err := strconv.Atoi(status)
		if err != nil {
			return err
		}
		return state.responseStatusShouldBe(expected)
	})
	ctx.Step(`^the response header "([^"]*)" should match "([^"]*)"$`, state.responseHeaderShouldMatchLocationTemplate)
	ctx.Step(`^response JSON field "([^"]*)" should equal "([^"]*)"$`, state.responseJSONFieldShouldEqual)
}
