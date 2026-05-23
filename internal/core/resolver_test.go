package core

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolverAgent_Execute(t *testing.T) {
	tests := []struct {
		name             string
		llmResponse      string
		llmErr           error
		expectedResolved bool
		expectErr        bool
	}{
		{
			name:             "resolves the issue",
			llmResponse:      "Here are the steps to fix your issue: 1. Do this 2. Do that",
			expectedResolved: true,
		},
		{
			name:             "escalates when cannot resolve",
			llmResponse:      "ESCALATE",
			expectedResolved: false,
		},
		{
			name:             "escalates when response contains ESCALATE",
			llmResponse:      "I cannot help with this. ESCALATE",
			expectedResolved: false,
		},
		{
			name:      "returns error on llm failure",
			llmErr:    errors.New("llm error"),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			llm := &mockLLM{
				response: tt.llmResponse,
				err:      tt.llmErr,
			}
			agent := NewResolverAgent(llm)
			state := State{
				SessionID: "test-session",
				Query:     "I cannot login",
				Category:  CategoryTechnical,
			}

			// Act
			result, err := agent.Execute(context.Background(), state)

			// Assert
			if tt.expectErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedResolved, result.NextState.Resolved)
			assert.Equal(t, AgentResolver, result.AgentType)
			assert.Equal(t, 10, result.TokensUsed)
		})
	}
}

func TestResolverAgent_Type(t *testing.T) {
	agent := NewResolverAgent(&mockLLM{})
	assert.Equal(t, AgentResolver, agent.Type())
}

func TestResolverAgent_DoesNotMutateOriginalState(t *testing.T) {
	llm := &mockLLM{response: "Here is the solution"}
	agent := NewResolverAgent(llm)

	original := State{
		SessionID: "test-session",
		Query:     "technical question",
		Category:  CategoryTechnical,
		Resolved:  false,
	}

	_, err := agent.Execute(context.Background(), original)
	require.NoError(t, err)

	// original state must never be mutated
	assert.False(t, original.Resolved)
	assert.Empty(t, original.Response)
}

func TestResolverAgent_SetsResponseOnResolve(t *testing.T) {
	expectedResponse := "Here are the steps to fix your issue"
	llm := &mockLLM{response: expectedResponse}
	agent := NewResolverAgent(llm)

	state := State{
		SessionID: "test-session",
		Query:     "how do I fix this?",
		Category:  CategoryTechnical,
	}

	result, err := agent.Execute(context.Background(), state)
	require.NoError(t, err)

	assert.True(t, result.NextState.Resolved)
	assert.Equal(t, expectedResponse, result.NextState.Response)
}