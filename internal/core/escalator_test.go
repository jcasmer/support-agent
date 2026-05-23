package core

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEscalatorAgent_Execute(t *testing.T) {
	validTicketJSON := `{
		"ticket_id": "TKT-20240523-001",
		"priority": "high",
		"summary": "User cannot login",
		"details": "User is getting 401 error on login",
		"suggested_team": "engineering",
		"next_steps": "Check auth service logs"
	}`

	tests := []struct {
		name             string
		llmResponse      string
		llmErr           error
		expectedTicketID string
		expectErr        bool
	}{
		{
			name:             "creates ticket successfully",
			llmResponse:      validTicketJSON,
			expectedTicketID: "TKT-20240523-001",
		},
		{
			name:        "handles json wrapped in code fences",
			llmResponse: "```json\n" + validTicketJSON + "\n```",
			expectedTicketID: "TKT-20240523-001",
		},
		{
			name:        "returns error on llm failure",
			llmErr:      errors.New("llm error"),
			expectErr:   true,
		},
		{
			name:        "returns error on invalid json",
			llmResponse: "this is not json",
			expectErr:   true,
		},
		{
			name:        "returns error on missing ticket_id",
			llmResponse: `{"priority": "high", "summary": "test"}`,
			expectErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			llm := &mockLLM{
				response: tt.llmResponse,
				err:      tt.llmErr,
			}
			agent := NewEscalatorAgent(llm)
			state := State{
				SessionID: "test-session",
				Query:     "I cannot login",
				Category:  CategoryTechnical,
				Resolved:  false,
			}

			// Act
			result, err := agent.Execute(context.Background(), state)

			// Assert
			if tt.expectErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedTicketID, result.NextState.TicketID)
			assert.Equal(t, AgentEscalator, result.AgentType)
			assert.NotEmpty(t, result.NextState.Response)
			assert.Contains(t, result.NextState.Response, tt.expectedTicketID)
		})
	}
}

func TestEscalatorAgent_Type(t *testing.T) {
	agent := NewEscalatorAgent(&mockLLM{})
	assert.Equal(t, AgentEscalator, agent.Type())
}

func TestEscalatorAgent_DoesNotMutateOriginalState(t *testing.T) {
	validTicketJSON := `{
		"ticket_id": "TKT-001",
		"priority": "low",
		"summary": "test",
		"details": "test details",
		"suggested_team": "engineering",
		"next_steps": "check logs"
	}`

	llm := &mockLLM{response: validTicketJSON}
	agent := NewEscalatorAgent(llm)

	original := State{
		SessionID: "test-session",
		Query:     "technical question",
		Category:  CategoryTechnical,
		TicketID:  "",
	}

	_, err := agent.Execute(context.Background(), original)
	require.NoError(t, err)

	// original state must never be mutated
	assert.Empty(t, original.TicketID)
	assert.Empty(t, original.Response)
}