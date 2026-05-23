package app

import (
	"context"
	"errors"
	"testing"

	"github.com/jcasmer/support-agent/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAgent is a test double for core.Agent
type mockAgent struct {
	agentType  core.AgentType
	nextState  core.State
	err        error
	called     bool
}

func (m *mockAgent) Type() core.AgentType {
	return m.agentType
}

func (m *mockAgent) Execute(_ context.Context, state core.State) (core.AgentResult, error) {
	m.called = true
	if m.err != nil {
		return core.AgentResult{}, m.err
	}
	return core.AgentResult{
		NextState:  m.nextState,
		AgentType:  m.agentType,
		TokensUsed: 10,
	}, nil
}

func TestOrchestrator_Run_ResolvedFlow(t *testing.T) {
	// Arrange
	classifierState := core.State{
		SessionID: "test-session",
		Query:     "how do I reset my password?",
		Category:  core.CategoryGeneral,
	}

	resolvedState := core.State{
		SessionID: "test-session",
		Query:     "how do I reset my password?",
		Category:  core.CategoryGeneral,
		Resolved:  true,
		Response:  "Here are the steps to reset your password",
		TokensUsed: 20,
	}

	classifier := &mockAgent{agentType: core.AgentClassifier, nextState: classifierState}
	resolver := &mockAgent{agentType: core.AgentResolver, nextState: resolvedState}
	escalator := &mockAgent{agentType: core.AgentEscalator}

	orch := New(classifier, resolver, escalator)

	// Act
	result, err := orch.Run(context.Background(), "test-session", "how do I reset my password?")

	// Assert
	require.NoError(t, err)
	assert.True(t, result.Resolved)
	assert.Equal(t, "Here are the steps to reset your password", result.Response)
	assert.Equal(t, string(core.CategoryGeneral), result.Category)
	assert.Empty(t, result.TicketID)

	// Escalator must NOT be called when resolved
	assert.False(t, escalator.called)
}

func TestOrchestrator_Run_EscalatedFlow(t *testing.T) {
	// Arrange
	classifierState := core.State{
		SessionID: "test-session",
		Query:     "complex technical issue",
		Category:  core.CategoryTechnical,
	}

	unresolvedState := core.State{
		SessionID: "test-session",
		Query:     "complex technical issue",
		Category:  core.CategoryTechnical,
		Resolved:  false,
		TokensUsed: 20,
	}

	escalatedState := core.State{
		SessionID:  "test-session",
		Query:      "complex technical issue",
		Category:   core.CategoryTechnical,
		Resolved:   false,
		TicketID:   "TKT-001",
		Response:   "Your issue has been escalated",
		TokensUsed: 30,
	}

	classifier := &mockAgent{agentType: core.AgentClassifier, nextState: classifierState}
	resolver := &mockAgent{agentType: core.AgentResolver, nextState: unresolvedState}
	escalator := &mockAgent{agentType: core.AgentEscalator, nextState: escalatedState}

	orch := New(classifier, resolver, escalator)

	// Act
	result, err := orch.Run(context.Background(), "test-session", "complex technical issue")

	// Assert
	require.NoError(t, err)
	assert.False(t, result.Resolved)
	assert.Equal(t, "TKT-001", result.TicketID)
	assert.Equal(t, "Your issue has been escalated", result.Response)

	// Escalator MUST be called when unresolved
	assert.True(t, escalator.called)
}

func TestOrchestrator_Run_ClassifierError(t *testing.T) {
	classifier := &mockAgent{
		agentType: core.AgentClassifier,
		err:       errors.New("classifier failed"),
	}
	resolver := &mockAgent{agentType: core.AgentResolver}
	escalator := &mockAgent{agentType: core.AgentEscalator}

	orch := New(classifier, resolver, escalator)

	_, err := orch.Run(context.Background(), "test-session", "some query")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "classification step")

	// neither resolver nor escalator should be called
	assert.False(t, resolver.called)
	assert.False(t, escalator.called)
}

func TestOrchestrator_Run_ResolverError(t *testing.T) {
	classifier := &mockAgent{
		agentType: core.AgentClassifier,
		nextState: core.State{Category: core.CategoryTechnical},
	}
	resolver := &mockAgent{
		agentType: core.AgentResolver,
		err:       errors.New("resolver failed"),
	}
	escalator := &mockAgent{agentType: core.AgentEscalator}

	orch := New(classifier, resolver, escalator)

	_, err := orch.Run(context.Background(), "test-session", "some query")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolution step")

	// escalator should not be called on resolver error
	assert.False(t, escalator.called)
}