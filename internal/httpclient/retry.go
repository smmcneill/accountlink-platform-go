package httpclient

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
)

type (
	HTTPRetryOption func(*httpRetryConfig)

	httpRetryConfig struct {
		retryMax int
		waitMin  time.Duration
		waitMax  time.Duration
		timeout  time.Duration
		client   *http.Client
		logger   *slog.Logger
	}

	failsafeRetryTransport struct {
		base   http.RoundTripper
		policy retrypolicy.RetryPolicy[struct{}]
		logger *slog.Logger
	}

	retryableStatusError struct {
		status int
	}
)

func NewHTTPRetryClient(opts ...HTTPRetryOption) *http.Client {
	cfg := httpRetryConfig{
		retryMax: 3,
		waitMin:  200 * time.Millisecond,
		waitMax:  2 * time.Second,
		logger:   slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	baseClient := cfg.client
	if baseClient == nil {
		baseClient = &http.Client{}
	}

	if cfg.timeout > 0 {
		baseClient.Timeout = cfg.timeout
	}

	baseTransport := baseClient.Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}

	builder := retrypolicy.NewBuilder[struct{}]().
		HandleIf(func(_ struct{}, err error) bool {
			return err != nil
		}).
		WithMaxRetries(cfg.retryMax).
		ReturnLastFailure()

	if cfg.waitMin > 0 && cfg.waitMax > cfg.waitMin {
		builder = builder.WithBackoff(cfg.waitMin, cfg.waitMax)
	} else if cfg.waitMin > 0 {
		builder = builder.WithDelay(cfg.waitMin)
	}

	rt := &failsafeRetryTransport{
		base:   baseTransport,
		policy: builder.Build(),
		logger: cfg.logger,
	}

	cloned := *baseClient
	cloned.Transport = rt
	return &cloned
}

func WithRetryMax(retryMax int) HTTPRetryOption {
	return func(cfg *httpRetryConfig) {
		cfg.retryMax = retryMax
	}
}

func WithRetryWaitMin(wait time.Duration) HTTPRetryOption {
	return func(cfg *httpRetryConfig) {
		cfg.waitMin = wait
	}
}

func WithRetryWaitMax(wait time.Duration) HTTPRetryOption {
	return func(cfg *httpRetryConfig) {
		cfg.waitMax = wait
	}
}

func WithHTTPTimeout(timeout time.Duration) HTTPRetryOption {
	return func(cfg *httpRetryConfig) {
		cfg.timeout = timeout
	}
}

func WithHTTPClient(client *http.Client) HTTPRetryOption {
	return func(cfg *httpRetryConfig) {
		cfg.client = client
	}
}

func WithLogger(logger *slog.Logger) HTTPRetryOption {
	return func(cfg *httpRetryConfig) {
		if logger == nil {
			logger = slog.New(slog.NewTextHandler(io.Discard, nil))
		}
		cfg.logger = logger
	}
}

func (t *failsafeRetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	attempt := 0
	var lastResp *http.Response

	_, err := failsafe.With(t.policy).Get(func() (struct{}, error) {
		attempt++
		tryReq, cloneErr := cloneRequest(req)
		if cloneErr != nil {
			return struct{}{}, cloneErr
		}

		if attempt > 1 {
			t.logger.Info("http retry attempt", "attempt", attempt, "method", req.Method, "url", req.URL.String())
		}

		//nolint:bodyclose // Body is either returned to caller or closed on retry/error paths.
		result, callErr := t.base.RoundTrip(tryReq)
		if callErr != nil {
			if result != nil {
				drainAndClose(result.Body)
			}
			return struct{}{}, callErr
		}

		if result != nil && result.StatusCode >= http.StatusInternalServerError {
			if lastResp != nil && lastResp.Body != result.Body {
				drainAndClose(lastResp.Body)
			}
			lastResp = result
			return struct{}{}, retryableStatusError{status: result.StatusCode}
		}
		lastResp = result
		return struct{}{}, nil
	})
	if err != nil {
		var statusErr retryableStatusError
		if errors.As(err, &statusErr) && lastResp != nil {
			return lastResp, nil
		}

		var exceeded retrypolicy.ExceededError
		if errors.As(err, &exceeded) {
			if errors.As(exceeded.LastError, &statusErr) && lastResp != nil {
				return lastResp, nil
			}
			if exceeded.LastError != nil {
				return nil, exceeded.LastError
			}
		}
		return nil, err
	}

	if lastResp == nil {
		return nil, fmt.Errorf("retry transport returned no response")
	}

	return lastResp, nil
}

func cloneRequest(req *http.Request) (*http.Request, error) {
	if req.Body == nil || req.GetBody != nil {
		return req.Clone(req.Context()), nil
	}

	return nil, fmt.Errorf("request body is not replayable for retries")
}

func drainAndClose(body io.ReadCloser) {
	if body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, body)
	_ = body.Close()
}

func (e retryableStatusError) Error() string {
	return fmt.Sprintf("retryable status: %d", e.status)
}
