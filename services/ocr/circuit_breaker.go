package ocr

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// CircuitBreakerState represents the operational state of the circuit breaker.
type CircuitBreakerState int

const (
	// StateClosed allows requests to pass to the upstream engine normally.
	StateClosed CircuitBreakerState = 0

	// StateHalfOpen allows a limited number of canary probe requests to test upstream recovery.
	StateHalfOpen CircuitBreakerState = 1

	// StateOpen fast-fails incoming requests immediately with ErrCircuitOpen without hitting upstream.
	StateOpen CircuitBreakerState = 2
)

// String returns a human-readable representation of CircuitBreakerState.
func (s CircuitBreakerState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateHalfOpen:
		return "half-open"
	case StateOpen:
		return "open"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// Global Prometheus metrics for Circuit Breaker.
var (
	defaultMetricBreakerState = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "lensio_ocr_circuit_breaker_state",
		Help: "Current state of the OCR circuit breaker (0=closed, 1=half-open, 2=open)",
	})
	defaultMetricBreakerTripped = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "lensio_ocr_circuit_breaker_tripped_total",
		Help: "Total number of times the OCR circuit breaker has tripped to open state",
	})
	registerOnce sync.Once
)

func registerDefaultMetrics() {
	registerOnce.Do(func() {
		_ = prometheus.Register(defaultMetricBreakerState)
		_ = prometheus.Register(defaultMetricBreakerTripped)
	})
}

// CircuitBreakerConfig controls the behavior of the circuit breaker.
type CircuitBreakerConfig struct {
	// FailureThreshold is the consecutive failure count required to trip to StateOpen (default: 5).
	FailureThreshold int

	// SuccessThreshold is the consecutive success count in StateHalfOpen to return to StateClosed (default: 2).
	SuccessThreshold int

	// Cooldown is the duration the breaker stays in StateOpen before transitioning to StateHalfOpen (default: 10s).
	Cooldown time.Duration

	// Timeout is the execution deadline applied to downstream calls (default: 5s, 0 disables).
	Timeout time.Duration

	// AdaptiveTimeout tightens the execution timeout dynamically as consecutive failures mount.
	AdaptiveTimeout bool

	// MinTimeout is the lower bound floor for adaptive timeout (default: 500ms).
	MinTimeout time.Duration

	// IsFailureFunc determines whether an error returned by the engine is an upstream failure.
	// Defaults to DefaultIsFailure (filters out client input errors).
	IsFailureFunc func(err error) bool

	// NowFunc provides current time (defaults to time.Now, useful for deterministic testing).
	NowFunc func() time.Time

	// OnStateChange is an optional callback invoked when the state transitions.
	OnStateChange func(from, to CircuitBreakerState)

	// Registerer is an optional Prometheus registerer. If nil, uses default Prometheus registry.
	Registerer prometheus.Registerer
}

// DefaultIsFailure determines if an error counts as an upstream OCR failure.
// User input validation errors (ErrInvalidDocument, ErrUnsupportedDocument, ErrLowConfidence)
// are client-side errors and do NOT trip the circuit breaker.
func DefaultIsFailure(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrInvalidDocument) ||
		errors.Is(err, ErrUnsupportedDocument) ||
		errors.Is(err, ErrLowConfidence) {
		return false
	}
	return true
}

// DefaultCircuitBreakerConfig returns standard production defaults.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Cooldown:         10 * time.Second,
		Timeout:          5 * time.Second,
		AdaptiveTimeout:  false,
		MinTimeout:       500 * time.Millisecond,
		IsFailureFunc:    DefaultIsFailure,
		NowFunc:          time.Now,
	}
}

// CircuitBreaker wraps an OCREngine with adaptive resilience and fast-fail protection.
type CircuitBreaker struct {
	engine OCREngine
	cfg    CircuitBreakerConfig

	mu                   sync.RWMutex
	state                CircuitBreakerState
	consecutiveFailures  int
	consecutiveSuccesses int
	halfOpenInFlight     int
	lastStateChange      time.Time

	metricState   prometheus.Gauge
	metricTripped prometheus.Counter
}

// NewCircuitBreaker creates a new CircuitBreaker wrapping the given OCREngine.
func NewCircuitBreaker(engine OCREngine, cfgs ...CircuitBreakerConfig) *CircuitBreaker {
	cfg := DefaultCircuitBreakerConfig()
	if len(cfgs) > 0 {
		userCfg := cfgs[0]
		if userCfg.FailureThreshold > 0 {
			cfg.FailureThreshold = userCfg.FailureThreshold
		}
		if userCfg.SuccessThreshold > 0 {
			cfg.SuccessThreshold = userCfg.SuccessThreshold
		}
		if userCfg.Cooldown > 0 {
			cfg.Cooldown = userCfg.Cooldown
		}
		if userCfg.Timeout > 0 {
			cfg.Timeout = userCfg.Timeout
		}
		cfg.AdaptiveTimeout = userCfg.AdaptiveTimeout
		if userCfg.MinTimeout > 0 {
			cfg.MinTimeout = userCfg.MinTimeout
		}
		if userCfg.IsFailureFunc != nil {
			cfg.IsFailureFunc = userCfg.IsFailureFunc
		}
		if userCfg.NowFunc != nil {
			cfg.NowFunc = userCfg.NowFunc
		}
		if userCfg.OnStateChange != nil {
			cfg.OnStateChange = userCfg.OnStateChange
		}
		cfg.Registerer = userCfg.Registerer
	}

	cb := &CircuitBreaker{
		engine: engine,
		cfg:    cfg,
		state:  StateClosed,
	}
	cb.lastStateChange = cb.now()

	if cfg.Registerer != nil {
		cb.metricState = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "lensio_ocr_circuit_breaker_state",
			Help: "Current state of the OCR circuit breaker (0=closed, 1=half-open, 2=open)",
		})
		cb.metricTripped = prometheus.NewCounter(prometheus.CounterOpts{
			Name: "lensio_ocr_circuit_breaker_tripped_total",
			Help: "Total number of times the OCR circuit breaker has tripped to open state",
		})
		_ = cfg.Registerer.Register(cb.metricState)
		_ = cfg.Registerer.Register(cb.metricTripped)
	} else {
		registerDefaultMetrics()
		cb.metricState = defaultMetricBreakerState
		cb.metricTripped = defaultMetricBreakerTripped
	}

	cb.metricState.Set(float64(StateClosed))
	return cb
}

func (cb *CircuitBreaker) now() time.Time {
	if cb.cfg.NowFunc != nil {
		return cb.cfg.NowFunc()
	}
	return time.Now()
}

// State returns the current CircuitBreakerState.
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Counts returns the current consecutive failures and consecutive successes.
func (cb *CircuitBreaker) Counts() (failures, successes int) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.consecutiveFailures, cb.consecutiveSuccesses
}

// Reset resets the circuit breaker to StateClosed with zero counters.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.transitionTo(StateClosed)
}

// Trip manually trips the circuit breaker to StateOpen.
func (cb *CircuitBreaker) Trip() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.transitionTo(StateOpen)
}

func (cb *CircuitBreaker) transitionTo(next CircuitBreakerState) {
	prev := cb.state
	if prev == next {
		return
	}
	cb.state = next
	cb.lastStateChange = cb.now()
	cb.consecutiveFailures = 0
	cb.consecutiveSuccesses = 0
	cb.halfOpenInFlight = 0

	if cb.metricState != nil {
		cb.metricState.Set(float64(next))
	}
	if next == StateOpen && cb.metricTripped != nil {
		cb.metricTripped.Inc()
	}
	if cb.cfg.OnStateChange != nil {
		cb.cfg.OnStateChange(prev, next)
	}
}

// currentTimeout calculates the adaptive execution timeout based on current consecutive failures.
func (cb *CircuitBreaker) currentTimeout() time.Duration {
	if cb.cfg.Timeout <= 0 {
		return 0
	}
	if !cb.cfg.AdaptiveTimeout || cb.cfg.FailureThreshold <= 1 {
		return cb.cfg.Timeout
	}

	cb.mu.RLock()
	failures := cb.consecutiveFailures
	cb.mu.RUnlock()

	if failures <= 0 {
		return cb.cfg.Timeout
	}
	if failures >= cb.cfg.FailureThreshold {
		failures = cb.cfg.FailureThreshold - 1
	}

	minT := cb.cfg.MinTimeout
	if minT <= 0 || minT >= cb.cfg.Timeout {
		minT = cb.cfg.Timeout / 2
	}

	delta := (cb.cfg.Timeout - minT) * time.Duration(failures) / time.Duration(cb.cfg.FailureThreshold)
	effective := cb.cfg.Timeout - delta
	if effective < minT {
		return minT
	}
	return effective
}

// beforeCall checks if the request is permitted by the circuit breaker.
func (cb *CircuitBreaker) beforeCall() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := cb.now()

	switch cb.state {
	case StateClosed:
		return nil

	case StateOpen:
		// Check if cooldown timer has elapsed
		if now.Sub(cb.lastStateChange) >= cb.cfg.Cooldown {
			cb.transitionTo(StateHalfOpen)
			cb.halfOpenInFlight = 1
			return nil
		}
		return ErrCircuitOpen

	case StateHalfOpen:
		// Canary probe limit: allow single request at a time
		if cb.halfOpenInFlight >= 1 {
			return ErrCircuitOpen
		}
		cb.halfOpenInFlight++
		return nil

	default:
		return nil
	}
}

// afterCall records the outcome and transitions state if thresholds are met.
func (cb *CircuitBreaker) afterCall(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	isFailure := cb.cfg.IsFailureFunc(err)

	switch cb.state {
	case StateClosed:
		if isFailure {
			cb.consecutiveFailures++
			if cb.consecutiveFailures >= cb.cfg.FailureThreshold {
				cb.transitionTo(StateOpen)
			}
		} else {
			cb.consecutiveFailures = 0
		}

	case StateHalfOpen:
		cb.halfOpenInFlight--
		if cb.halfOpenInFlight < 0 {
			cb.halfOpenInFlight = 0
		}

		if isFailure {
			// Any failure during half-open immediately re-trips to Open
			cb.transitionTo(StateOpen)
		} else {
			cb.consecutiveSuccesses++
			if cb.consecutiveSuccesses >= cb.cfg.SuccessThreshold {
				cb.transitionTo(StateClosed)
			}
		}

	case StateOpen:
		// If in open state, nothing to update
	}
}

// Execute wraps an arbitrary function call within circuit breaker protection.
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(ctx context.Context) (*OCRResult, error)) (*OCRResult, error) {
	if err := cb.beforeCall(); err != nil {
		return nil, err
	}

	execTimeout := cb.currentTimeout()
	var cancel context.CancelFunc
	execCtx := ctx
	if execTimeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, execTimeout)
		defer cancel()
	}

	res, err := fn(execCtx)
	cb.afterCall(err)
	return res, err
}

// Extract implements the OCREngine interface.
func (cb *CircuitBreaker) Extract(ctx context.Context, image []byte) (*OCRResult, error) {
	if cb.engine == nil {
		return nil, ErrOCRFailed
	}
	return cb.Execute(ctx, func(c context.Context) (*OCRResult, error) {
		return cb.engine.Extract(c, image)
	})
}
